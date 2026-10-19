//go:build windows

package proc

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tyzrex/kport/types"
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

	cache := map[int]types.Process{}
	for i := range results {
		p, ok := cache[results[i].PID]
		if !ok {
			name, cmd, exe, user, ppid, cwd, rss := GetProcessDetails(results[i].PID)
			p = types.Process{Name: name, CommandLine: cmd, Exe: exe, User: user, PPID: ppid, Cwd: cwd, MemoryRSS: rss, MemoryHuman: FormatBytes(rss)}
			cache[results[i].PID] = p
		}
		results[i].Name = p.Name
		results[i].CommandLine = p.CommandLine
		results[i].Exe = p.Exe
		results[i].User = p.User
		results[i].PPID = p.PPID
		results[i].Cwd = p.Cwd
		results[i].MemoryRSS = p.MemoryRSS
		results[i].MemoryHuman = p.MemoryHuman
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
