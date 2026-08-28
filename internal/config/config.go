package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents user-defined preferences and custom presets.
type Config struct {
	Presets        map[string][]int `json:"presets,omitempty"`
	ProtectedPorts []int            `json:"protected_ports,omitempty"`
	DefaultForce   bool             `json:"default_force,omitempty"`
	DefaultTree    bool             `json:"default_tree,omitempty"`
	DefaultTimeout string           `json:"default_timeout,omitempty"`
	Timeout        time.Duration    `json:"-"`
}

// LoadConfig reads the configuration file from ~/.config/killport/config.json or ~/.killport.json.
func LoadConfig() Config {
	var cfg Config
	cfg.Presets = make(map[string][]int)

	paths := getCandidateConfigPaths()
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &cfg)
			break
		}
	}

	if cfg.DefaultTimeout != "" {
		if dur, err := time.ParseDuration(cfg.DefaultTimeout); err == nil {
			cfg.Timeout = dur
		}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 1500 * time.Millisecond
	}

	// Lowercase all preset keys for case-insensitive lookup
	normPresets := make(map[string][]int)
	for k, v := range cfg.Presets {
		normPresets[strings.ToLower(k)] = v
	}
	cfg.Presets = normPresets

	return cfg
}

func getCandidateConfigPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".killport.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "killport", "config.json"),
			filepath.Join(home, ".killport.json"),
		)
	}
	return paths
}
