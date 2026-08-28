package proc

import (
	"github.com/tyzrex/kport/types"
)

// FindProcesses returns processes listening on the given ports.
func FindProcesses(ports []int, opts types.FindOptions) ([]types.Process, error) {
	return findOSProcesses(ports, opts)
}

// FindAllProcesses returns all listening processes on the system.
func FindAllProcesses(opts types.FindOptions) ([]types.Process, error) {
	return findOSProcesses(nil, opts)
}
