package tui

import (
	"github.com/tyzrex/kport/types"
	"reflect"
	"strings"
	"testing"
)

func TestDecoderPreservesSplitInputAndPaste(t *testing.T) {
	var d decoder
	if got := d.feed("\x1b["); len(got) != 0 {
		t.Fatal(got)
	}
	if got := d.feed("B/fastify"); len(got) != 9 || got[0].name != "down" || got[2].text != "f" {
		t.Fatalf("%+v", got)
	}
	if got := d.feed("\xe7\x8c"); len(got) != 0 {
		t.Fatal(got)
	}
	if got := d.feed("\xab"); len(got) != 1 || got[0].text != "猫" {
		t.Fatal(got)
	}
	if got := d.feed("\x1b[200~Fxyq\x1b[20"); len(got) != 0 {
		t.Fatal(got)
	}
	if got := d.feed("1~"); !reflect.DeepEqual(got, []key{{name: "paste", text: "Fxyq"}}) {
		t.Fatalf("paste interpreted as commands: %+v", got)
	}
	if got := d.feed("\x1b[99~"); len(got) != 0 {
		t.Fatalf("unsupported CSI became keys: %+v", got)
	}
	d.feed("\x1b")
	if got := d.escape(); len(got) != 1 || got[0].name != "esc" {
		t.Fatal(got)
	}
}
func TestSelectionGroupsSocketsAndSurvivesRefresh(t *testing.T) {
	ps := []types.Process{{PID: 10, StartID: "old", Name: "node", Port: 3000}, {PID: 10, StartID: "old", Name: "node", Port: 3001}, {PID: 11, StartID: "other", Name: "postgres", Port: 5432}}
	m := dashboard{selected: map[string]bool{}, stopped: map[string]types.Process{}, sort: "port", exposure: "all"}
	m.update(ps)
	if len(m.groups) != 2 || len(m.groups[0].sockets) != 2 {
		t.Fatal(m.groups)
	}
	m.selected[m.groups[0].key] = true
	m.query = "postgres"
	targets := m.targets(m.visible())
	if len(targets) != 2 {
		t.Fatalf("hidden selection lost: %+v", targets)
	}
	m.update(ps)
	if !m.selected[owner(ps[0])] {
		t.Fatal("selection lost after refresh")
	}
	ps[0].StartID = "new"
	ps[1].StartID = "new"
	m.update(ps)
	if m.selected[owner(ps[0])] {
		t.Fatal("selection transferred to reused PID")
	}
}
func TestFilterExposureAndSort(t *testing.T) {
	gs := groupProcesses([]types.Process{{PID: 2, Name: "fastify", Cwd: "/work/api", User: "alice", Port: 3000, Address: "[::1]:3000", MemoryRSS: 20}, {PID: 3, Name: "redis", Port: 6379, Address: "*:6379", MemoryRSS: 30}})
	if got := filterGroups(gs, "alice api 3000", "localhost"); len(got) != 1 {
		t.Fatal(got)
	}
	sortGroups(gs, "memory")
	if gs[0].process.PID != 3 {
		t.Fatal(gs)
	}
	for addr, want := range map[string]string{"[::]:80": "public", ":::80": "public", "0.0.0.0:80": "public", "127.0.0.2:80": "localhost", "[::1]:80": "localhost", "192.168.1.2:80": "interface"} {
		if got := Exposure(addr); got != want {
			t.Errorf("%s got %s want %s", addr, got, want)
		}
	}
	if strings.Contains(Clean("bad\x1b[2J\n"), "\x1b") {
		t.Fatal("terminal control escaped sanitization")
	}
	if got := backspace("abc猫"); got != "abc" {
		t.Fatal(got)
	}
	if got := clip("猫abc", 3); got != "猫a" {
		t.Fatal(got)
	}
}
func TestContainersGroupIndependentlyOfSharedBackend(t *testing.T) {
	ps := []types.Process{{PID: 10, ContainerID: "a", Port: 3000}, {PID: 10, ContainerID: "b", Port: 4000}, {PID: 10, ContainerID: "a", Port: 3001}}
	gs := groupProcesses(ps)
	if len(gs) != 2 || len(gs[0].sockets) != 2 {
		t.Fatal(gs)
	}
}
