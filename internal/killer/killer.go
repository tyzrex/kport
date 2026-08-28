package killer

import (
	"errors"
	"os"

	"github.com/tyzrex/killport/internal/proc"
	"github.com/tyzrex/killport/types"
)

var (
	ErrProtectedPID = errors.New("cannot kill protected system process (PID <= 1)")
	ErrSelfPID      = errors.New("cannot kill self process")
)

// ValidatePID ensures the target PID is safe to kill.
func ValidatePID(pid int) error {
	if pid <= 1 {
		return ErrProtectedPID
	}
	if pid == os.Getpid() {
		return ErrSelfPID
	}
	return nil
}

// KillProcess terminates a process according to the provided options, including child trees if requested.
func KillProcess(p types.Process, opts types.KillOptions) types.KillResult {
	if opts.DryRun {
		return types.KillResult{
			Port:    p.Port,
			Process: p,
			Killed:  false,
			Signal:  "DRY-RUN",
			Error:   nil,
		}
	}

	if err := ValidatePID(p.PID); err != nil {
		return types.KillResult{
			Port:    p.Port,
			Process: p,
			Killed:  false,
			Error:   err,
		}
	}

	var childrenKilled int
	// Kill descendant process tree if requested
	if opts.KillTree {
		children := proc.FindProcessChildren(p.PID)
		for _, childPID := range children {
			if childPID > 1 && childPID != os.Getpid() {
				if killed, _, _ := terminateProcess(childPID, opts); killed {
					childrenKilled++
				}
			}
		}
	}

	killed, sig, err := terminateProcess(p.PID, opts)
	return types.KillResult{
		Port:           p.Port,
		Process:        p,
		Killed:         killed,
		Signal:         sig,
		ChildrenKilled: childrenKilled,
		Error:          err,
	}
}
