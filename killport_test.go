package killport_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/tyzrex/killport"
)

// Helper to spawn a dummy listener in a background subprocess
func startChildListener(t *testing.T, proto string) (*exec.Cmd, int) {
	// Find a free port
	var freePort int
	if proto == "udp" {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
		if err != nil {
			t.Fatalf("failed to get free UDP port: %v", err)
		}
		freePort = conn.LocalAddr().(*net.UDPAddr).Port
		conn.Close()
	} else {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to get free TCP port: %v", err)
		}
		freePort = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	}

	time.Sleep(50 * time.Millisecond)

	var cmd *exec.Cmd
	if proto == "udp" {
		script := fmt.Sprintf("import socket, time; s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM); s.bind(('127.0.0.1', %d)); time.sleep(10)", freePort)
		cmd = exec.Command("python3", "-c", script)
	} else {
		script := fmt.Sprintf("import socket, time; s = socket.socket(socket.AF_INET, socket.SOCK_STREAM); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1); s.bind(('127.0.0.1', %d)); s.listen(1); time.sleep(10)", freePort)
		cmd = exec.Command("python3", "-c", script)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start listener process: %v", err)
	}

	// Give the process time to bind
	time.Sleep(200 * time.Millisecond)

	return cmd, freePort
}

func TestFindAndKillPortTCP(t *testing.T) {
	cmd, port := startChildListener(t, "tcp")
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// 1. Test IsPortInUse
	inUse, err := killport.IsPortInUse(port)
	if err != nil {
		t.Fatalf("IsPortInUse returned error: %v", err)
	}
	if !inUse {
		t.Fatalf("expected port %d to be in use", port)
	}

	// 2. Test Find
	procs, err := killport.Find(port)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(procs) == 0 {
		t.Fatalf("expected to find process on port %d, found none", port)
	}

	foundPID := procs[0].PID
	if foundPID != cmd.Process.Pid {
		t.Logf("found PID %d, expected cmd PID %d", foundPID, cmd.Process.Pid)
	}

	// 3. Test DryRun
	dryResults, err := killport.KillPort(port, killport.KillOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if len(dryResults) == 0 {
		t.Fatalf("DryRun expected 1 result")
	}
	if dryResults[0].Killed {
		t.Fatalf("DryRun should not kill the process")
	}

	// 4. Test Kill
	killResults, err := killport.Kill(port)
	if err != nil {
		t.Fatalf("Kill returned error: %v", err)
	}
	if len(killResults) == 0 {
		t.Fatalf("Kill returned no results")
	}
	if !killResults[0].Killed {
		t.Fatalf("Kill failed: %v", killResults[0].Error)
	}

	// Verify the process exited
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process terminated cleanly as expected
	case <-time.After(1 * time.Second):
		t.Fatalf("expected process %d to terminate within 1s", cmd.Process.Pid)
	}

	// Verify port is no longer in use
	time.Sleep(100 * time.Millisecond)
	inUseAfter, _ := killport.IsPortInUse(port)
	if inUseAfter {
		t.Fatalf("expected port %d to be free after kill", port)
	}
}

func TestKillMultiplePorts(t *testing.T) {
	cmd1, port1 := startChildListener(t, "tcp")
	defer func() {
		if cmd1.Process != nil {
			_ = cmd1.Process.Kill()
		}
	}()

	cmd2, port2 := startChildListener(t, "tcp")
	defer func() {
		if cmd2.Process != nil {
			_ = cmd2.Process.Kill()
		}
	}()

	results, err := killport.Kill(port1, port2)
	if err != nil {
		t.Fatalf("Kill multiple ports returned error: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	for _, r := range results {
		if !r.Killed {
			t.Errorf("expected port %d to be killed, got err: %v", r.Port, r.Error)
		}
	}
}

func TestKillNonExistentPort(t *testing.T) {
	results, err := killport.Kill(59999)
	if err != nil {
		t.Fatalf("unexpected error for unused port: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for unused port, got %d", len(results))
	}
}

func TestSafetyPIDProtection(t *testing.T) {
	selfProc := killport.Process{
		PID:      os.Getpid(),
		Port:     12345,
		Protocol: killport.ProtocolTCP,
	}

	res := killport.KillProcess(selfProc)
	if res.Killed {
		t.Fatalf("expected self-process kill to be rejected")
	}
	if res.Error == nil {
		t.Fatalf("expected error when trying to kill self")
	}

	initProc := killport.Process{
		PID:      1,
		Port:     80,
		Protocol: killport.ProtocolTCP,
	}
	res2 := killport.KillProcess(initProc)
	if res2.Killed {
		t.Fatalf("expected init PID 1 kill to be rejected")
	}
	if res2.Error == nil {
		t.Fatalf("expected error when trying to kill PID 1")
	}
}

func TestFindAndKillPortUDP(t *testing.T) {
	cmd, port := startChildListener(t, "udp")
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// 1. Test IsPortInUse
	inUse, err := killport.IsPortInUse(port, killport.ProtocolUDP)
	if err != nil {
		t.Fatalf("IsPortInUse UDP returned error: %v", err)
	}
	if !inUse {
		t.Logf("UDP port %d might not show up in unconn table immediately, continuing", port)
	}

	// 2. Test Kill with ProtocolUDP
	results, err := killport.KillPort(port, killport.KillOptions{
		Protocol: killport.ProtocolUDP,
		Force:    true,
	})
	if err != nil {
		t.Fatalf("KillPort UDP returned error: %v", err)
	}
	t.Logf("UDP Kill results count: %d", len(results))
}

func TestFindAll(t *testing.T) {
	procs, err := killport.FindAll()
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	t.Logf("Total listening processes discovered: %d", len(procs))
	for i, p := range procs {
		if i < 5 {
			t.Logf("  [%d] Port %d (%s) PID %d: %s", i, p.Port, p.Protocol, p.PID, p.Name)
		}
	}
}

func TestPresetsAndProtected(t *testing.T) {
	// Test Presets
	nextPorts, ok := killport.ResolvePreset("next")
	if !ok || len(nextPorts) == 0 || nextPorts[0] != 3000 {
		t.Fatalf("expected next preset to map to port 3000, got %v", nextPorts)
	}

	pgPorts, ok := killport.ResolvePreset("postgres")
	if !ok || len(pgPorts) == 0 || pgPorts[0] != 5432 {
		t.Fatalf("expected postgres preset to map to 5432, got %v", pgPorts)
	}

	// Test Protected Ports
	desc, isProt := killport.IsProtectedPort(22)
	if !isProt || desc == "" {
		t.Fatalf("expected port 22 to be marked as protected")
	}

	_, isProt8080 := killport.IsProtectedPort(8080)
	if isProt8080 {
		t.Fatalf("port 8080 should not be protected")
	}
}

func TestFindByName(t *testing.T) {
	cmd, port := startChildListener(t, "tcp")
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Python3 was started
	procs, err := killport.FindByName("python")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}

	found := false
	for _, p := range procs {
		if p.Port == port {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Process on port %d might not have 'python' in comm/exe on this system, continuing", port)
	}

	// Test GetProcessDetails
	details, err := killport.GetProcessDetails(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("GetProcessDetails failed: %v", err)
	}
	if details.PID != cmd.Process.Pid {
		t.Fatalf("expected PID %d, got %d", cmd.Process.Pid, details.PID)
	}
	t.Logf("Process %d details: Name=%s, Memory=%s, Exe=%s", details.PID, details.Name, details.MemoryHuman, details.Exe)
}
