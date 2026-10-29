package proc

import (
	"context"

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
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if err := opts.Context.Err(); err != nil {
		return nil, err
	}
	ps, err := findOSProcesses(ports, opts)
	if opts.Context.Err() != nil {
		return nil, opts.Context.Err()
	}
	if err != nil {
		return nil, err
	}
	ids := map[int]string{}
	var result []types.Process
	for _, p := range ps {
		if err := opts.Context.Err(); err != nil {
			return nil, err
		}
		if !MatchesNameFilter(p, opts.NameFilter) {
			continue
		}
		id, ok := ids[p.PID]
		if !ok {
			id = p.StartID
			if id == "" {
				id = StartIdentity(p.PID)
			}
			ids[p.PID] = id
		}
		p.StartID = id
		if opts.IncludeTree && p.ChildrenPIDs == nil {
			p.ChildrenPIDs = FindProcessChildrenContext(opts.Context, p.PID)
		}
		result = append(result, p)
	}
	if err := opts.Context.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// descendants returns leaf-first targets with cycle protection.
func descendants(pid int, parents map[int][]int) []int {
	result := []int{}
	seen := map[int]bool{pid: true}
	var visit func(int)
	visit = func(p int) {
		for _, child := range parents[p] {
			if !seen[child] {
				seen[child] = true
				visit(child)
				result = append(result, child)
			}
		}
	}
	visit(pid)
	return result
}
