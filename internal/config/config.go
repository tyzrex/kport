package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Presets        map[string][]int `json:"presets,omitempty"`
	ProtectedPorts []int            `json:"protected_ports,omitempty"`
	DefaultForce   bool             `json:"default_force,omitempty"`
	DefaultTree    bool             `json:"default_tree,omitempty"`
	DefaultTimeout string           `json:"default_timeout,omitempty"`
	Timeout        time.Duration    `json:"-"`
}

// Load merges global preferences with project overrides, from ancestors to CWD.
func Load() (Config, error) {
	cfg := Config{Presets: map[string][]int{}}
	var paths []string
	if home, e := os.UserHomeDir(); e == nil {
		paths = append(paths, filepath.Join(home, ".killport.json"), filepath.Join(home, ".config", "killport", "config.json"))
	}
	if cwd, e := os.Getwd(); e == nil {
		var local []string
		for d := cwd; ; d = filepath.Dir(d) {
			local = append(local, filepath.Join(d, ".killport.json"))
			if filepath.Dir(d) == d {
				break
			}
		}
		for i := len(local) - 1; i >= 0; i-- {
			paths = append(paths, local[i])
		}
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		b, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return cfg, e
		}
		var raw map[string]json.RawMessage
		if e = json.Unmarshal(b, &raw); e != nil {
			return cfg, fmt.Errorf("%s: %w", p, e)
		}
		// Unmarshal onto the current config to preserve unspecified scalar defaults.
		presets := make(map[string][]int)
		for k, v := range cfg.Presets {
			presets[strings.ToLower(k)] = v
		}
		protected := append([]int(nil), cfg.ProtectedPorts...)
		cfg.Presets = nil // Decode only this file's preset keys before merging case-insensitively.
		if e = json.Unmarshal(b, &cfg); e != nil {
			return cfg, fmt.Errorf("%s: %w", p, e)
		}
		for k, v := range cfg.Presets {
			presets[strings.ToLower(k)] = v
		}
		cfg.Presets = presets
		if _, ok := raw["protected_ports"]; ok {
			cfg.ProtectedPorts = append(protected, cfg.ProtectedPorts...)
		}
	}
	cfg.Timeout = 1500 * time.Millisecond
	if cfg.DefaultTimeout != "" {
		d, e := time.ParseDuration(cfg.DefaultTimeout)
		if e != nil || d <= 0 {
			return cfg, fmt.Errorf("default_timeout must be a positive duration")
		}
		cfg.Timeout = d
	}
	for _, ports := range cfg.Presets {
		for _, p := range ports {
			if p < 1 || p > 65535 {
				return cfg, fmt.Errorf("invalid preset port %d", p)
			}
		}
	}
	for _, p := range cfg.ProtectedPorts {
		if p < 1 || p > 65535 {
			return cfg, fmt.Errorf("invalid protected port %d", p)
		}
	}
	return cfg, nil
}
