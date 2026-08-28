package killport

import (
	"fmt"

	"github.com/tyzrex/killport/internal/killer"
	"github.com/tyzrex/killport/internal/proc"
)

// Kill kills processes listening on one or more specified ports using default options.
// Example:
//
//	results, err := killport.Kill(3000)
//	results, err := killport.Kill(3000, 8080, 5432)
func Kill(ports ...int) ([]KillResult, error) {
	return KillPorts(ports, DefaultKillOptions())
}

// KillPort kills processes listening on a single port with custom options.
// Example:
//
//	results, err := killport.KillPort(3000, killport.KillOptions{Force: true})
func KillPort(port int, opts KillOptions) ([]KillResult, error) {
	return KillPorts([]int{port}, opts)
}

// KillPorts kills processes listening on the given list of ports with custom options.
// Example:
//
//	results, err := killport.KillPorts([]int{3000, 8080}, killport.KillOptions{
//	    Protocol: killport.ProtocolTCP,
//	    Force:    true,
//	})
func KillPorts(ports []int, opts KillOptions) ([]KillResult, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports specified to kill")
	}

	// 1. Discover all processes on the specified ports
	processes, err := proc.FindProcesses(ports, FindOptions{Protocol: opts.Protocol})
	if err != nil {
		return nil, fmt.Errorf("failed to discover processes on ports %v: %w", ports, err)
	}

	if len(processes) == 0 {
		return nil, nil
	}

	// 2. Kill each discovered process
	var results []KillResult
	killedPIDs := make(map[int]bool)

	for _, p := range processes {
		// If a process was already killed in this batch (e.g. listening on both IPv4 and IPv6 or multiple ports),
		// report it accordingly without error
		if killedPIDs[p.PID] {
			results = append(results, KillResult{
				Port:    p.Port,
				Process: p,
				Killed:  true,
				Signal:  "ALREADY_KILLED",
				Error:   nil,
			})
			continue
		}

		res := killer.KillProcess(p, opts)
		if res.Killed {
			killedPIDs[p.PID] = true
		}
		results = append(results, res)
	}

	return results, nil
}

// Find returns all active processes listening on the given ports using default options.
// Example:
//
//	procs, err := killport.Find(3000)
//	procs, err := killport.Find(3000, 8080)
func Find(ports ...int) ([]Process, error) {
	return FindPorts(ports, DefaultFindOptions())
}

// FindPort returns processes listening on a single port with custom options.
func FindPort(port int, opts FindOptions) ([]Process, error) {
	return FindPorts([]int{port}, opts)
}

// FindPorts returns processes listening on multiple ports with custom options.
func FindPorts(ports []int, opts FindOptions) ([]Process, error) {
	return proc.FindProcesses(ports, opts)
}

// FindAll returns all listening processes across all ports on the system.
func FindAll(opts ...FindOptions) ([]Process, error) {
	opt := DefaultFindOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	return proc.FindAllProcesses(opt)
}

// IsPortInUse returns true if at least one process is listening on the specified port.
func IsPortInUse(port int, protocol ...Protocol) (bool, error) {
	proto := ProtocolAll
	if len(protocol) > 0 {
		proto = protocol[0]
	}
	procs, err := FindPort(port, FindOptions{Protocol: proto})
	if err != nil {
		return false, err
	}
	return len(procs) > 0, nil
}

// KillProcess kills a specific Process instance directly.
func KillProcess(p Process, opts ...KillOptions) KillResult {
	opt := DefaultKillOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	return killer.KillProcess(p, opt)
}
