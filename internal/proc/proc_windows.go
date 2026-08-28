//go:build windows

package proc

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tyzrex/killport/types"
)

func findOSProcesses(ports []int, opts types.FindOptions) ([]types.Process, error) {
	opts.Protocol = opts.Protocol.Normalize()
	var results []types.Process

	if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolTCP {
		cmd := exec.Command("netstat", "-ano", "-p", "tcp")
		if out, err := cmd.Output(); err == nil {
			results = append(results, parseWindowsNetstat(out, ports, types.ProtocolTCP)...)
		}
	}

	if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolUDP {
		cmd := exec.Command("netstat", "-ano", "-p", "udp")
		if out, err := cmd.Output(); err == nil {
			results = append(results, parseWindowsNetstat(out, ports, types.ProtocolUDP)...)
		}
	}

	// Populate process names and command lines
	for i := range results {
		name, cmdline := getWindowsProcessInfo(results[i].PID)
		results[i].Name = name
		results[i].CommandLine = cmdline
	}

	return DeduplicateProcesses(results), nil
}

func parseWindowsNetstat(out []byte, ports []int, proto types.Protocol) []types.Process {
	var results []types.Process
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Active") || strings.HasPrefix(line, "Proto") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// TCP line: Proto LocalAddress ForeignAddress State PID (len 5)
		// UDP line: Proto LocalAddress ForeignAddress PID (len 4)
		var localAddr string
		var state string
		var pidStr string

		if proto == types.ProtocolTCP {
			if len(fields) < 5 {
				continue
			}
			localAddr = fields[1]
			state = fields[3]
			pidStr = fields[4]

			if state != "LISTENING" {
				continue
			}
		} else {
			localAddr = fields[1]
			state = "BOUND"
			pidStr = fields[3]
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}

		idx := strings.LastIndex(localAddr, ":")
		if idx == -1 {
			continue
		}

		portStr := localAddr[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		if !MatchPort(ports, port) {
			continue
		}

		results = append(results, types.Process{
			PID:      pid,
			Port:     port,
			Protocol: proto,
			Address:  localAddr,
			State:    state,
		})
	}

	return results
}

func getWindowsProcessInfo(pid int) (string, string) {
	if pid <= 0 {
		return "", ""
	}
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH", "/FI", fmt.Sprintf("PID eq %d", pid))
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	line := strings.TrimSpace(string(out))
	if line == "" || strings.Contains(line, "No tasks") {
		return "", ""
	}

	parts := strings.Split(line, ",")
	if len(parts) > 0 {
		name := strings.Trim(parts[0], "\"")
		return name, ""
	}

	return "", ""
}
