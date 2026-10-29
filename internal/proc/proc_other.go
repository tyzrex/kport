//go:build !linux && !darwin && !windows

package proc

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tyzrex/kport/types"
)

func findOSProcesses(ports []int, opts types.FindOptions) ([]types.Process, error) {
	opts.Protocol = opts.Protocol.Normalize()
	lsofPath, err := exec.LookPath("lsof")
	if err != nil {
		return nil, fmt.Errorf("lsof not found: %w", err)
	}

	var results []types.Process
	for _, port := range ports {
		args := []string{fmt.Sprintf("-i:%d", port), "-sTCP:LISTEN", "-n", "-P", "-F", "pcn"}
		out, err := Output(opts.Context, lsofPath, args...)
		if err != nil || len(out) == 0 {
			continue
		}

		lines := strings.Split(string(out), "\n")
		var currentPID int
		var currentName string
		var currentAddr string

		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			switch line[0] {
			case 'p':
				if currentPID > 0 {
					results = append(results, types.Process{
						PID:      currentPID,
						Name:     currentName,
						Port:     port,
						Protocol: types.ProtocolTCP,
						Address:  currentAddr,
						State:    "LISTEN",
					})
				}
				currentPID, _ = strconv.Atoi(line[1:])
			case 'c':
				currentName = line[1:]
			case 'n':
				currentAddr = line[1:]
			}
		}

		if currentPID > 0 {
			results = append(results, types.Process{
				PID:      currentPID,
				Name:     currentName,
				Port:     port,
				Protocol: types.ProtocolTCP,
				Address:  currentAddr,
				State:    "LISTEN",
			})
		}
	}

	return DeduplicateProcesses(results), nil
}
