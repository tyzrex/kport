package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tyzrex/kport"
)

// Rewrite os.Args for the real command entry point, in an isolated subprocess.
func TestMain(m *testing.M) {
	if os.Getenv("KPORT_CLI_HELPER") == "1" {
		for i, a := range os.Args {
			if a == "--" {
				os.Args = append([]string{"kport"}, os.Args[i+1:]...)
				Run("test")
				os.Exit(0)
			}
		}
	}
	os.Exit(m.Run())
}
func TestListenerHelper(t *testing.T) {
	if os.Getenv("KPORT_CLI_LISTENER") != "1" {
		return
	}
	address := os.Getenv("KPORT_CLI_BIND")
	if address == "" {
		address = "127.0.0.1:0"
	}
	ln, e := net.Listen("tcp", address)
	if e != nil {
		os.Exit(2)
	}
	defer ln.Close()
	if childFile := os.Getenv("KPORT_CLI_CHILD_FILE"); childFile != "" {
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestListenerHelper$")
		child.Env = append(os.Environ(), "KPORT_CLI_CHILD_FILE=", "KPORT_CLI_BIND=127.0.0.1:0")
		pipe, e := child.StdoutPipe()
		if e != nil {
			os.Exit(2)
		}
		if child.Start() != nil {
			os.Exit(2)
		}
		s := bufio.NewScanner(pipe)
		if !s.Scan() {
			os.Exit(2)
		}
		if os.WriteFile(childFile, []byte(fmt.Sprintf("%s %d", s.Text(), child.Process.Pid)), 0600) != nil {
			os.Exit(2)
		}
		go child.Wait()
	}
	fmt.Println(ln.Addr())
	for {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		c.Close()
	}
}
func command(t *testing.T, dir string, args ...string) *exec.Cmd {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, exe, append([]string{"-test.run=^TestCommandHelper$", "--"}, args...)...)
	home := t.TempDir()
	cmd.Env = append(os.Environ(), "KPORT_CLI_HELPER=1", "HOME="+home, "USERPROFILE="+home)
	cmd.Dir = dir
	return cmd
}
func listener(t *testing.T, dir string) (*exec.Cmd, int) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestListenerHelper$")
	cmd.Env = append(os.Environ(), "KPORT_CLI_LISTENER=1")
	cmd.Dir = dir
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	cmd.Stderr = os.Stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	ready := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(pipe)
		if s.Scan() {
			ready <- s.Text()
		} else {
			ready <- ""
		}
	}()
	select {
	case addr := <-ready:
		_, p, e := net.SplitHostPort(addr)
		if e != nil {
			t.Fatal(e)
		}
		port, _ := strconv.Atoi(p)
		return cmd, port
	case <-time.After(10 * time.Second):
		t.Fatal("listener not ready")
	}
	return nil, 0
}
func assertListening(t *testing.T, port int) {
	t.Helper()
	c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if e != nil {
		t.Fatalf("listener unexpectedly stopped: %v", e)
	}
	c.Close()
}
func TestReadOnlyCommandsDoNotKill(t *testing.T) {
	dir := t.TempDir()
	_, port := listener(t, dir)
	for _, mode := range []string{"--inspect", "--dry-run", "--list"} {
		t.Run(mode, func(t *testing.T) {
			cmd := command(t, dir, strconv.Itoa(port), mode, "--json")
			out, e := cmd.Output()
			if e != nil {
				t.Fatal(e)
			}
			var ps []killport.Process
			if e = json.Unmarshal(out, &ps); e != nil || len(ps) != 1 {
				t.Fatalf("%s: %s (%v)", mode, out, e)
			}
			assertListening(t, port)
		})
	}
	for _, args := range [][]string{{"--watch", "--dry-run"}, {"--observe", "--inspect"}} {
		cmd := command(t, dir, append(args, strconv.Itoa(port))...)
		if e := cmd.Run(); e == nil {
			t.Fatal("conflicting watch mode accepted")
		}
		assertListening(t, port)
	}
}
func TestConfiguredProtectionRequiresExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	_, port := listener(t, dir)
	os.WriteFile(filepath.Join(dir, ".killport.json"), []byte(fmt.Sprintf(`{"protected_ports":[%d],"default_timeout":"2s","default_tree":true}`, port)), 0600)
	out, e := command(t, dir, strconv.Itoa(port), "--force", "--json").CombinedOutput()
	if e == nil || !bytes.Contains(out, []byte("protected")) {
		t.Fatalf("force bypassed protection: %s %v", out, e)
	}
	assertListening(t, port)
	cmd := command(t, dir, strconv.Itoa(port), "--force-protected", "--json", "--wait")
	out, e = cmd.Output()
	if e != nil {
		t.Fatalf("override failed: %s %v", out, e)
	}
	var results []struct {
		Killed bool
		Error  string
	}
	if json.Unmarshal(out, &results) != nil || len(results) == 0 || !results[0].Killed || results[0].Error != "" {
		t.Fatalf("%s", out)
	}
}
func TestProjectDoesNotKillUnrelatedDefaultPort(t *testing.T) {
	project, other := t.TempDir(), t.TempDir()
	_, port := listener(t, other)
	os.WriteFile(filepath.Join(project, "package.json"), []byte(fmt.Sprintf(`{"scripts":{"dev":"next dev --port %d"},"dependencies":{"next":"*"}}`, port)), 0600)
	out, e := command(t, project, ".", "--json").Output()
	if e != nil {
		t.Fatal(e)
	}
	if strings.TrimSpace(string(out)) != "[]" {
		t.Fatalf("unrelated process selected: %s", out)
	}
	assertListening(t, port)
	if runtime.GOOS == "windows" {
		t.Skip("Windows CWD ownership is unavailable")
	}
	_, ownPort := listener(t, project)
	cmd := command(t, project, ".", "--json", "--wait")
	out, e = cmd.Output()
	if e != nil {
		t.Fatalf("project kill failed: %s %v", out, e)
	}
	assertListening(t, port)
	c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ownPort), time.Second)
	if e == nil {
		c.Close()
		t.Fatal("owned process still listening")
	}
}
func TestObserveNeverStopsListener(t *testing.T) {
	dir := t.TempDir()
	_, port := listener(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	count := 0
	o := options{observe: true, protocol: "tcp", exposure: "all", interval: 10 * time.Millisecond}
	timer := time.AfterFunc(10*time.Second, cancel)
	defer timer.Stop()
	e := watchLoop(ctx, []int{port}, o, func(ev watchEvent) {
		if ev.Event == "occupied" {
			count++
			cancel()
		}
	})
	if e != nil || count != 1 {
		t.Fatalf("observe: count=%d err=%v", count, e)
	}
	assertListening(t, port)
}
func TestInvalidArgumentsFail(t *testing.T) {
	for _, args := range [][]string{{",,,"}, {" "}, {"--list", "typo"}, {"--protocol", "sctp", "3000"}, {"--timeout", "0", "3000"}, {"--interval", "bad", "--watch", "3000"}, {"--watch"}, {"free", "70000"}, {"free", "9000-8000"}} {
		if out, e := command(t, t.TempDir(), args...).CombinedOutput(); e == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
}

func TestTreeStopsDescendantListener(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "child.txt")
	t.Setenv("KPORT_CLI_CHILD_FILE", file)
	_, port := listener(t, dir)
	data, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Fields(string(data))
	if len(parts) != 2 {
		t.Fatal(string(data))
	}
	_, childPortStr, e := net.SplitHostPort(parts[0])
	if e != nil {
		t.Fatal(e)
	}
	childPort, _ := strconv.Atoi(childPortStr)
	childPID, _ := strconv.Atoi(parts[1])
	childDetails, e := killport.GetProcessDetails(childPID)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if now, e := killport.GetProcessDetails(childPID); e == nil && now.StartID == childDetails.StartID {
			p, _ := os.FindProcess(childPID)
			p.Kill()
		}
	})
	out, e := command(t, dir, strconv.Itoa(port), "--tree", "--force", "--wait", "--json").Output()
	if e != nil {
		t.Fatalf("tree stop: %s %v", out, e)
	}
	var rs []struct {
		ChildrenKilled int `json:"children_killed"`
	}
	if json.Unmarshal(out, &rs) != nil || len(rs) == 0 || rs[0].ChildrenKilled < 1 {
		t.Fatalf("child not stopped: %s", out)
	}
	if c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", childPort), time.Second); e == nil {
		c.Close()
		t.Fatal("descendant still listening")
	}
}
func TestWatchReportsRespawnAndHonorsCancellation(t *testing.T) {
	dir := t.TempDir()
	_, port := listener(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stops, respawns := 0, 0
	o := options{protocol: "tcp", exposure: "all", interval: 10 * time.Millisecond, timeout: time.Second}
	e := watchLoop(ctx, []int{port}, o, func(ev watchEvent) {
		if ev.Event == "stopped" {
			stops++
			if stops == 1 {
				t.Setenv("KPORT_CLI_BIND", fmt.Sprintf("127.0.0.1:%d", port))
				listener(t, dir)
			}
		}
		if ev.Event == "respawned" {
			respawns++
			if ev.Process == nil || ev.Process.PPID == 0 {
				t.Error("respawn lacks parent metadata")
			}
			cancel()
		}
	})
	if e != nil || stops != 1 || respawns != 1 {
		t.Fatalf("watch: stops=%d respawns=%d err=%v", stops, respawns, e)
	}
	assertListening(t, port)
}
