package app

import (
	"context"
	"errors"
	"testing"

	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/config"
)

func TestSessionConfigurationIsIsolated(t *testing.T) {
	cfg := config.Config{Presets: map[string][]int{"custom": {4100}}, ProtectedPorts: []int{4100}}
	a, b := New(cfg), New(config.Config{})
	cfg.Presets["custom"][0] = 9999
	ports, ok := a.ResolvePreset("CUSTOM")
	if !ok || len(ports) != 1 || ports[0] != 4100 {
		t.Fatal(ports, ok)
	}
	ports[0] = 9998
	ports, _ = a.ResolvePreset("custom")
	if ports[0] != 4100 {
		t.Fatal("caller modified session presets")
	}
	if _, ok := b.ResolvePreset("custom"); ok {
		t.Fatal("preset leaked to another session")
	}
	target := killport.Process{PID: 999998, Port: 4100}
	if a.CheckProtection(target, nil, false, false) == nil {
		t.Fatal("configured protection ignored")
	}
	if err := b.CheckProtection(target, nil, false, false); err != nil {
		t.Fatal("protection leaked:", err)
	}
	if _, ok := killport.Presets["custom"]; ok {
		t.Fatal("global presets mutated")
	}
}

func TestCancelledActionsDoNotSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New(config.Config{})
	if _, err := s.Discover(ctx, killport.FindOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	rs := s.ExecuteTargets([]killport.Process{{PID: 999998, StartID: "test", Port: 4100}}, killport.KillOptions{Context: ctx}, false)
	if len(rs) != 1 || !errors.Is(rs[0].Error, context.Canceled) || rs[0].Killed {
		t.Fatal(rs)
	}
}

func TestDockerEnrichmentRespectsQuery(t *testing.T) {
	ps := []killport.Process{
		{ContainerID: "a", Port: 3000, Protocol: killport.ProtocolTCP},
		{ContainerID: "b", Port: 3000, Protocol: killport.ProtocolUDP},
		{ContainerID: "c", Port: 4000, Protocol: killport.ProtocolTCP},
	}
	got := restrictDiscovery(ps, []int{3000}, killport.ProtocolTCP)
	if len(got) != 1 || got[0].ContainerID != "a" {
		t.Fatal(got)
	}
	if got := restrictDiscovery(ps, nil, killport.ProtocolAll); len(got) != 3 {
		t.Fatal(got)
	}
}
