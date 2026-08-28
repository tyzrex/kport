package killer

import (
	"errors"
	"os"

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

// KillProcess terminates a process according to the provided options.
func KillProcess(proc types.Process, opts types.KillOptions) types.KillResult {
	if opts.DryRun {
		return types.KillResult{
			Port:    proc.Port,
			Process: proc,
			Killed:  false,
			Signal:  "DRY-RUN",
			Error:   nil,
		}
	}

	if err := ValidatePID(proc.PID); err != nil {
		return types.KillResult{
			Port:    proc.Port,
			Process: proc,
			Killed:  false,
			Error:   err,
		}
	}

	killed, sig, err := terminateProcess(proc.PID, opts)
	return types.KillResult{
		Port:    proc.Port,
		Process: proc,
		Killed:  killed,
		Signal:  sig,
		Error:   err,
	}
}
