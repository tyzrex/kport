//go:build darwin

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
	var pid int
	var name string
	cache := map[int]types.Process{}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			name = line[1:]
		case 'n':
			addr := line[1:]
			if strings.Contains(addr, "->") {
				continue
			}
			i := strings.LastIndex(addr, ":")
			if i < 0 {
				continue
			}
			port, e := strconv.Atoi(addr[i+1:])
			if e != nil || port == 0 {
				continue
			}
			if defaultPort != 0 && port != defaultPort {
				continue
			}
			p, ok := cache[pid]
			if !ok {
				n, cmd, exe, user, ppid, cwd, rss := GetProcessDetails(pid)
				if n == "" {
					n = name
				}
				p = types.Process{PID: pid, Name: n, CommandLine: cmd, Exe: exe, User: user, PPID: ppid, Cwd: cwd, MemoryRSS: rss, MemoryHuman: FormatBytes(rss)}
				cache[pid] = p
			}
			p.Port = port
			p.Protocol = proto
			p.Address = addr
			p.State = "LISTEN"
			if proto == types.ProtocolUDP {
				p.State = "BOUND"
			}
			results = append(results, p)
		}
	}
	return results
}
