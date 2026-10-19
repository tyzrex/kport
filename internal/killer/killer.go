package killer

import (
	"errors"
	"fmt"
	"os"

	"github.com/tyzrex/kport/internal/proc"
	"github.com/tyzrex/kport/types"
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

	if p.StartID == "" {
		p.StartID = proc.StartIdentity(p.PID)
	}
	if p.StartID != "" && proc.StartIdentity(p.PID) != p.StartID {
		return types.KillResult{Port: p.Port, Process: p, Error: fmt.Errorf("process %d changed or exited; refresh before stopping", p.PID)}
	}
	var childrenKilled int
	var childErrors []error
	// Kill descendant process tree if requested
	if opts.KillTree {
		children := proc.FindProcessChildren(p.PID)
		for _, childPID := range children {
			if childPID > 1 && childPID != os.Getpid() {
				childOpts := opts
				childOpts.ExpectedStartID = proc.StartIdentity(childPID)
				if childOpts.ExpectedStartID == "" {
					childErrors = append(childErrors, fmt.Errorf("cannot verify child PID %d", childPID))
					continue
				}
				if killed, _, e := terminateProcess(childPID, childOpts); killed {
					childrenKilled++
				} else if e != nil {
					childErrors = append(childErrors, e)
				}
			}
		}
	}

	opts.ExpectedStartID = p.StartID
	killed, sig, err := terminateProcess(p.PID, opts)
	return types.KillResult{
		Port:           p.Port,
		Process:        p,
		Killed:         killed,
		Signal:         sig,
		ChildrenKilled: childrenKilled,
		Error:          errors.Join(append(childErrors, err)...),
	}
}
