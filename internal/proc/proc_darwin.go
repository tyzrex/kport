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
	for _, proto := range []types.Protocol{types.ProtocolTCP, types.ProtocolUDP} {
		if !opts.Protocol.Matches(proto) {
			continue
		}
		args := []string{"-n", "-P", "-F", "pcn"}
		selector := "-iTCP"
		if proto == types.ProtocolUDP {
			selector = "-iUDP"
		} else {
			args = append(args, "-sTCP:LISTEN")
		}
		if len(ports) > 0 {
			values := make([]string, len(ports))
			for i, port := range ports {
				values[i] = strconv.Itoa(port)
			}
			selector += ":" + strings.Join(values, ",")
		}
		out, err := lsofOutput(opts.Context, lsofPath, append(args, selector)...)
		if err != nil {
			return nil, fmt.Errorf("discover %s listeners: %w", proto, err)
		}
		results = append(results, parseDarwinLsofOutput(out, 0, proto)...)
	}
	results = DeduplicateProcesses(results)
	if len(results) == 0 {
		return results, nil
	}
	return enrichDarwin(opts, results)
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
				p = types.Process{PID: pid, Name: name}
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
