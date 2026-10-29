//go:build !windows

package killer

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/tyzrex/kport/internal/proc"
	"github.com/tyzrex/kport/types"
)

func TestPinnedTreeRejectsNewChildBeforeSignaling(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 60 & echo $!; wait")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal(err)
	}
	defer func() { _ = syscall.Kill(child, syscall.SIGKILL); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	p := types.Process{PID: cmd.Process.Pid, StartID: proc.StartIdentity(cmd.Process.Pid)}
	result := KillProcess(p, types.KillOptions{Force: true, KillTree: true, TreeTargets: map[int]string{}})
	if result.Error == nil || result.Killed || !strings.Contains(result.Error.Error(), "tree changed") {
		t.Fatal(result)
	}
	if syscall.Kill(cmd.Process.Pid, 0) != nil || syscall.Kill(child, 0) != nil {
		t.Fatal("unapproved tree was signaled")
	}
}
