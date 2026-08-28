//go:build darwin

package proc

import (
	"bytes"
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
		return nil, fmt.Errorf("lsof not found on darwin system: %w", err)
	}

	var results []types.Process

	if len(ports) == 0 {
		if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolTCP {
			cmd := exec.Command(lsofPath, "-iTCP", "-sTCP:LISTEN", "-n", "-P", "-F", "pcn")
			if out, err := cmd.Output(); err == nil {
				results = append(results, parseDarwinLsofOutput(out, 0, types.ProtocolTCP)...)
			}
		}
		if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolUDP {
			cmd := exec.Command(lsofPath, "-iUDP", "-n", "-P", "-F", "pcn")
			if out, err := cmd.Output(); err == nil {
				results = append(results, parseDarwinLsofOutput(out, 0, types.ProtocolUDP)...)
			}
		}
	} else {
		for _, port := range ports {
			if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolTCP {
				cmd := exec.Command(lsofPath, fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-n", "-P", "-F", "pcn")
				if out, err := cmd.Output(); err == nil {
					results = append(results, parseDarwinLsofOutput(out, port, types.ProtocolTCP)...)
				}
			}
			if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolUDP {
				cmd := exec.Command(lsofPath, fmt.Sprintf("-iUDP:%d", port), "-n", "-P", "-F", "pcn")
				if out, err := cmd.Output(); err == nil {
					results = append(results, parseDarwinLsofOutput(out, port, types.ProtocolUDP)...)
				}
			}
		}
	}

	return DeduplicateProcesses(results), nil
}

func parseDarwinLsofOutput(out []byte, defaultPort int, proto types.Protocol) []types.Process {
	var results []types.Process
	lines := strings.Split(string(out), "\n")

	var currentPID int
	var currentName string
	var currentAddr string

	flush := func() {
		if currentPID > 0 {
			port := defaultPort
			if port == 0 && currentAddr != "" {
				if idx := strings.LastIndex(currentAddr, ":"); idx != -1 {
					if p, err := strconv.Atoi(currentAddr[idx+1:]); err == nil {
						port = p
					}
				}
			}

			cmdline := getDarwinCmdline(currentPID)

			results = append(results, types.Process{
				PID:         currentPID,
				Name:        currentName,
				CommandLine: cmdline,
				Port:        port,
				Protocol:    proto,
				Address:     currentAddr,
				State:       "LISTEN",
			})
			currentPID = 0
			currentName = ""
			currentAddr = ""
		}
	}

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		prefix := line[0]
		val := line[1:]

		switch prefix {
		case 'p':
			flush()
			currentPID, _ = strconv.Atoi(val)
		case 'c':
			currentName = val
		case 'n':
			currentAddr = val
		}
	}
	flush()

	return results
}

func getDarwinCmdline(pid int) string {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		return strings.TrimSpace(out.String())
	}
	return ""
}
