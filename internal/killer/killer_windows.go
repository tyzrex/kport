//go:build windows

package killer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/tyzrex/kport/internal/proc"
	"github.com/tyzrex/kport/types"
	"golang.org/x/sys/windows"
)

func terminateProcess(pid int, opts types.KillOptions) (bool, string, error) {
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if e := opts.Context.Err(); e != nil {
		return false, "", e
	}
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
		_, _ = proc.Output(opts.Context, "taskkill", "/PID", strconv.Itoa(pid))
		if status, _ := waitForHandle(opts.Context, h, timeout); status == windows.WAIT_OBJECT_0 {
			return true, "CLOSE", nil
		}
	}
	if e := opts.Context.Err(); e != nil {
		return false, "CLOSE", e
	}
	if e = windows.TerminateProcess(h, 1); e != nil {
		return false, "TERMINATE", e
	}
	status, e := waitForHandle(opts.Context, h, time.Second)
	if e != nil {
		return false, "TERMINATE", e
	}
	if status != windows.WAIT_OBJECT_0 {
		return false, "TERMINATE", fmt.Errorf("PID %d has not exited", pid)
	}
	return true, "TERMINATE", nil
}

func waitForHandle(ctx context.Context, h windows.Handle, timeout time.Duration) (uint32, error) {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return uint32(windows.WAIT_TIMEOUT), err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return uint32(windows.WAIT_TIMEOUT), nil
		}
		status, err := windows.WaitForSingleObject(h, uint32(max(1, min(remaining.Milliseconds(), 50))))
		if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
			return status, err
		}
	}
}
