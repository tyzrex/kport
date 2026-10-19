package cli

import (
	"github.com/tyzrex/kport"
	"testing"
)

func TestProtectionCoversSiblingAndChildPorts(t *testing.T) {
	target := killport.Process{PID: 999998, StartID: "a", Port: 3000, ChildrenPIDs: []int{999997}}
	sibling := target
	sibling.Port = 22
	child := killport.Process{PID: 999997, Port: 53}
	if e := checkProtection(target, []killport.Process{sibling}, false, false); e == nil {
		t.Fatal("protected sibling port ignored")
	}
	if e := checkProtection(target, []killport.Process{child}, true, false); e == nil {
		t.Fatal("protected child port ignored")
	}
	if e := checkProtection(target, []killport.Process{sibling}, false, true); e != nil {
		t.Fatal(e)
	}
	all := []killport.Process{target, sibling, child}
	if got := expandTargets([]killport.Process{target}, all); len(got) != 2 {
		t.Fatal(got)
	}
}
func TestExpandRejectsPIDReuseAndSharedContainerBackend(t *testing.T) {
	target := killport.Process{PID: 10, StartID: "old", Port: 3000}
	replacement := target
	replacement.StartID = "new"
	replacement.Port = 4000
	expanded := expandTargets([]killport.Process{target}, []killport.Process{replacement})
	if len(expanded) != 1 || expanded[0].StartID != "old" {
		t.Fatal(expanded)
	}
	a := killport.Process{PID: 10, ContainerID: "a", Port: 3000}
	b := a
	b.ContainerID = "b"
	b.Port = 4000
	expanded = expandTargets([]killport.Process{a}, []killport.Process{a, b})
	if len(expanded) != 1 || expanded[0].ContainerID != "a" {
		t.Fatal(expanded)
	}
}
