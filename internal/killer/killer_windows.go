//go:build windows

package killer

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/tyzrex/killport/types"
)

func terminateProcess(pid int, opts types.KillOptions) (bool, string, error) {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	if err := cmd.Run(); err == nil {
		return true, "TERMINATE", nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, "", fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := proc.Kill(); err != nil {
		return false, "", fmt.Errorf("failed to kill PID %d: %w", pid, err)
	}

	time.Sleep(100 * time.Millisecond)
	return true, "TERMINATE", nil
}
