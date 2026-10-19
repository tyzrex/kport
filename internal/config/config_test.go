package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigMergesProjectOverridesAndValidates(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	old, _ := os.Getwd()
	if e := os.Chdir(project); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Chdir(old) })
	write := func(p, s string) {
		t.Helper()
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write(filepath.Join(home, ".killport.json"), `{"presets":{"API":[4100]},"protected_ports":[4100],"default_force":true,"default_tree":true,"default_timeout":"3s"}`)
	write(filepath.Join(project, ".killport.json"), `{"presets":{"web":[4200],"API":[4300]},"protected_ports":[4200],"default_force":false}`)
	cfg, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if cfg.DefaultForce || !cfg.DefaultTree || cfg.Timeout != 3*time.Second || len(cfg.ProtectedPorts) != 2 || cfg.Presets["api"][0] != 4300 || cfg.Presets["web"][0] != 4200 {
		t.Fatalf("%+v", cfg)
	}
	write(filepath.Join(project, ".killport.json"), `{"default_timeout":"bad"}`)
	if _, e = Load(); e == nil {
		t.Fatal("invalid duration ignored")
	}
	write(filepath.Join(project, ".killport.json"), `{broken`)
	if _, e = Load(); e == nil {
		t.Fatal("malformed config ignored")
	}
}
