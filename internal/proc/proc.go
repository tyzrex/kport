package proc

import (
	"github.com/tyzrex/kport/types"
)

// FindProcesses returns processes listening on the given ports.
func FindProcesses(ports []int, opts types.FindOptions) ([]types.Process, error) {
	return discover(ports, opts)
}

// FindAllProcesses returns all listening processes on the system.
func FindAllProcesses(opts types.FindOptions) ([]types.Process, error) {
	return discover(nil, opts)
}

func discover(ports []int, opts types.FindOptions) ([]types.Process, error) {
	ps, err := findOSProcesses(ports, opts)
	if err != nil {
		return nil, err
	}
	ids := map[int]string{}
	var result []types.Process
	for _, p := range ps {
		if !MatchesNameFilter(p, opts.NameFilter) {
			continue
		}
		id, ok := ids[p.PID]
		if !ok {
			id = StartIdentity(p.PID)
			ids[p.PID] = id
		}
		p.StartID = id
		if opts.IncludeTree {
			p.ChildrenPIDs = FindProcessChildren(p.PID)
		}
		result = append(result, p)
	}
	return result, nil
}
