//go:build windows

package killer

import (
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/tyzrex/kport/types"
	"golang.org/x/sys/windows"
)

func terminateProcess(pid int, opts types.KillOptions) (bool, string, error) {
	// Holding a process handle pins the identity through graceful timeout and
	// escalation, so a replacement process with the same PID is never targeted.
	h, e := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if e != nil {
		return false, "", fmt.Errorf("open PID %d: %w", pid, e)
	}
	defer windows.CloseHandle(h)
	if opts.ExpectedStartID != "" {
		var c, x, k, u windows.Filetime
		if e = windows.GetProcessTimes(h, &c, &x, &k, &u); e != nil {
			return false, "", e
		}
		if fmt.Sprintf("%d:%d", c.HighDateTime, c.LowDateTime) != opts.ExpectedStartID {
			return false, "", fmt.Errorf("process identity changed")
		}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if !opts.Force {
		// taskkill without /F requests normal window closure when supported. Some
		// console services cannot close gracefully and need the bounded fallback.
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid)).Run()
		if status, _ := windows.WaitForSingleObject(h, uint32(min(timeout.Milliseconds(), int64(0xfffffffe)))); status == windows.WAIT_OBJECT_0 {
			return true, "CLOSE", nil
		}
	}
	if e = windows.TerminateProcess(h, 1); e != nil {
		return false, "TERMINATE", e
	}
	status, e := windows.WaitForSingleObject(h, 1000)
	if e != nil {
		return false, "TERMINATE", e
	}
	if status != windows.WAIT_OBJECT_0 {
		return false, "TERMINATE", fmt.Errorf("PID %d has not exited", pid)
	}
	return true, "TERMINATE", nil
}
