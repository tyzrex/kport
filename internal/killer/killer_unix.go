//go:build !windows

package killer

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tyzrex/kport/internal/proc"
	"github.com/tyzrex/kport/types"
)

func terminateProcess(pid int, opts types.KillOptions) (bool, string, error) {
	// Check if process exists
	if !isProcessAlive(pid) {
		return false, "", fmt.Errorf("process %d does not exist", pid)
	}

	if opts.ExpectedStartID != "" && proc.StartIdentity(pid) != opts.ExpectedStartID {
		return false, "", fmt.Errorf("process identity changed")
	}

	// Force kill mode
	if opts.Force {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			if err == syscall.ESRCH {
				return true, "SIGKILL", nil
			}
			return false, "SIGKILL", fmt.Errorf("failed to send SIGKILL to PID %d: %w", pid, err)
		}
		// Confirm death
		if !waitForExit(pid, 200*time.Millisecond) {
			return false, "SIGKILL", fmt.Errorf("PID %d has not exited", pid)
		}
		return true, "SIGKILL", nil
	}

	// Graceful kill mode: send SIGTERM first
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return true, "SIGTERM", nil
		}
		return false, "SIGTERM", fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	// Wait for process to exit gracefully
	if exited := waitForExit(pid, timeout); exited {
		return true, "SIGTERM", nil
	}

	if opts.ExpectedStartID != "" && proc.StartIdentity(pid) != opts.ExpectedStartID {
		return true, "SIGTERM", nil
	}

	// Process still alive after timeout: send SIGKILL
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return true, "SIGKILL", nil
		}
		return false, "SIGKILL", fmt.Errorf("process did not exit after SIGTERM, SIGKILL failed for PID %d: %w", pid, err)
	}

	if !waitForExit(pid, 300*time.Millisecond) {
		return false, "SIGKILL", fmt.Errorf("PID %d has not exited", pid)
	}
	return true, "SIGKILL", nil
}

func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err != nil && err != syscall.EPERM {
		return false
	}
	if runtime.GOOS == "darwin" {
		out, e := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
		if e != nil || strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			return false
		}
	}
	// On Linux, check if process is in zombie or dead state
	if statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		lines := strings.Split(string(statusBytes), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "State:") {
				if strings.Contains(line, "Z (zombie)") || strings.Contains(line, "X (dead)") {
					return false
				}
				break
			}
		}
	}
	return true
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	interval := 25 * time.Millisecond

	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return true
		}
		time.Sleep(interval)
		if interval < 100*time.Millisecond {
			interval += 10 * time.Millisecond
		}
	}

	return !isProcessAlive(pid)
}
