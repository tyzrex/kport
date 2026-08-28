//go:build linux

package proc

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tyzrex/killport/types"
)

type socketInfo struct {
	inode    string
	port     int
	protocol types.Protocol
	address  string
	state    string
}

func findOSProcesses(ports []int, opts types.FindOptions) ([]types.Process, error) {
	opts.Protocol = opts.Protocol.Normalize()
	var results []types.Process

	// 1. Try pure /proc filesystem first (fastest, pure Go, zero subprocess overhead)
	procResults, err := scanProcNet(ports, opts)
	if err == nil && len(procResults) > 0 {
		results = append(results, procResults...)
	}

	// 2. If some ports were specifically requested and not all found, or if no results, try fallback tools
	missingPorts := getMissingPorts(ports, results)
	if len(ports) > 0 && len(missingPorts) > 0 {
		// Fallback: ss
		if ssResults, err := scanWithSS(missingPorts, opts); err == nil && len(ssResults) > 0 {
			results = append(results, ssResults...)
		}

		// Fallback: lsof
		missingPorts = getMissingPorts(ports, results)
		if len(missingPorts) > 0 {
			if lsofResults, err := scanWithLsof(missingPorts, opts); err == nil && len(lsofResults) > 0 {
				results = append(results, lsofResults...)
			}
		}

		// Fallback: fuser
		missingPorts = getMissingPorts(ports, results)
		if len(missingPorts) > 0 {
			if fuserResults, err := scanWithFuser(missingPorts, opts); err == nil && len(fuserResults) > 0 {
				results = append(results, fuserResults...)
			}
		}
	} else if len(ports) == 0 && len(results) == 0 {
		// If scanning all ports and /proc returned empty (e.g. restricted container environment)
		if ssResults, err := scanWithSS(nil, opts); err == nil && len(ssResults) > 0 {
			results = append(results, ssResults...)
		}
	}

	return DeduplicateProcesses(results), nil
}

func getMissingPorts(requested []int, found []types.Process) []int {
	if len(requested) == 0 {
		return nil
	}
	foundMap := make(map[int]bool)
	for _, p := range found {
		foundMap[p.Port] = true
	}
	var missing []int
	for _, p := range requested {
		if !foundMap[p] {
			missing = append(missing, p)
		}
	}
	return missing
}

func scanProcNet(ports []int, opts types.FindOptions) ([]types.Process, error) {
	inodeMap := make(map[string]socketInfo)

	// Scan TCP
	if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolTCP {
		parseProcNetFile("/proc/net/tcp", types.ProtocolTCP, false, ports, inodeMap)
		parseProcNetFile("/proc/net/tcp6", types.ProtocolTCP, true, ports, inodeMap)
	}

	// Scan UDP
	if opts.Protocol == types.ProtocolAll || opts.Protocol == types.ProtocolUDP {
		parseProcNetFile("/proc/net/udp", types.ProtocolUDP, false, ports, inodeMap)
		parseProcNetFile("/proc/net/udp6", types.ProtocolUDP, true, ports, inodeMap)
	}

	if len(inodeMap) == 0 {
		return nil, nil
	}

	// Map inodes to PIDs by scanning /proc/<pid>/fd
	return scanPIDsForInodes(inodeMap)
}

func parseProcNetFile(filePath string, proto types.Protocol, isIPv6 bool, ports []int, inodeMap map[string]socketInfo) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return // skip header
	}

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// fields[1] = local_address (hex_ip:hex_port)
		// fields[3] = st (state)
		// fields[9] = inode
		localAddr := fields[1]
		state := fields[3]
		inode := fields[9]

		if inode == "0" {
			continue
		}

		// For TCP, we want 0A (LISTEN). For UDP, 07 (CLOSE) represents bound/listening sockets.
		if proto == types.ProtocolTCP && state != "0A" {
			continue
		}

		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		port, err := ParseHexPort(portHex)
		if err != nil {
			continue
		}

		if !MatchPort(ports, port) {
			continue
		}

		var ipStr string
		if isIPv6 {
			ipStr = ParseHexIPv6(parts[0])
		} else {
			ipStr = ParseHexIPv4(parts[0])
		}

		stateStr := "LISTEN"
		if proto == types.ProtocolUDP {
			stateStr = "BOUND"
		}

		inodeMap[inode] = socketInfo{
			inode:    inode,
			port:     port,
			protocol: proto,
			address:  fmt.Sprintf("%s:%d", ipStr, port),
			state:    stateStr,
		}
	}
}

func scanPIDsForInodes(inodeMap map[string]socketInfo) ([]types.Process, error) {
	procDir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var results []types.Process

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry)
		if err != nil || pid <= 0 {
			continue
		}

		fdDirPath := filepath.Join("/proc", entry, "fd")
		fdDir, err := os.Open(fdDirPath)
		if err != nil {
			// Permission denied or process exited
			continue
		}

		fdEntries, err := fdDir.Readdirnames(-1)
		fdDir.Close()
		if err != nil {
			continue
		}

		for _, fdName := range fdEntries {
			fdPath := filepath.Join(fdDirPath, fdName)
			link, err := os.Readlink(fdPath)
			if err != nil {
				continue
			}

			// socket:[12345] or [0000]:12345
			var inode string
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inode = link[8 : len(link)-1]
			} else if strings.HasPrefix(link, "[0000]:") {
				inode = link[7:]
			}

			if inode == "" {
				continue
			}

			if sock, ok := inodeMap[inode]; ok {
				name, cmdline, exe, user := GetProcessDetails(pid)
				results = append(results, types.Process{
					PID:         pid,
					Name:        name,
					CommandLine: cmdline,
					Exe:         exe,
					User:        user,
					Port:        sock.port,
					Protocol:    sock.protocol,
					Address:     sock.address,
					State:       sock.state,
				})
			}
		}
	}

	return results, nil
}

// Fallback: ss -lptn / ss -lpun
func scanWithSS(ports []int, opts types.FindOptions) ([]types.Process, error) {
	ssPath, err := exec.LookPath("ss")
	if err != nil {
		return nil, err
	}

	var args []string
	if opts.Protocol == types.ProtocolTCP {
		args = []string{"-lptn"}
	} else if opts.Protocol == types.ProtocolUDP {
		args = []string{"-lpun"}
	} else {
		args = []string{"-lptun"}
	}

	cmd := exec.Command(ssPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseSSOutput(out, ports, opts.Protocol)
}

func parseSSOutput(out []byte, ports []int, targetProto types.Protocol) ([]types.Process, error) {
	var results []types.Process
	scanner := bufio.NewScanner(bytes.NewReader(out))

	procRegex := regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		proto := types.ProtocolTCP
		if strings.HasPrefix(fields[0], "udp") || strings.Contains(line, "UNCONN") {
			proto = types.ProtocolUDP
		}

		if !targetProto.Matches(proto) {
			continue
		}

		// Find local address & port
		var localAddr string
		for _, f := range fields {
			if strings.Contains(f, ":") && !strings.Contains(f, "users:") && !strings.Contains(f, "pid=") {
				localAddr = f
				break
			}
		}

		if localAddr == "" {
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

		// Extract PID & process name
		matches := procRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				procName := match[1]
				pid, err := strconv.Atoi(match[2])
				if err != nil {
					continue
				}

				name, cmdline, exe, user := GetProcessDetails(pid)
				if name == "" {
					name = procName
				}

				results = append(results, types.Process{
					PID:         pid,
					Name:        name,
					CommandLine: cmdline,
					Exe:         exe,
					User:        user,
					Port:        port,
					Protocol:    proto,
					Address:     localAddr,
					State:       "LISTEN",
				})
			}
		}
	}

	return results, nil
}

// Fallback: lsof -i :<port> -sTCP:LISTEN -n -P -F pcn
func scanWithLsof(ports []int, opts types.FindOptions) ([]types.Process, error) {
	lsofPath, err := exec.LookPath("lsof")
	if err != nil {
		return nil, err
	}

	var results []types.Process
	for _, port := range ports {
		var protoArgs []string
		if opts.Protocol == types.ProtocolTCP {
			protoArgs = []string{fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN"}
		} else if opts.Protocol == types.ProtocolUDP {
			protoArgs = []string{fmt.Sprintf("-iUDP:%d", port)}
		} else {
			protoArgs = []string{fmt.Sprintf("-i:%d", port)}
		}

		args := append(protoArgs, "-n", "-P", "-F", "pcn")
		cmd := exec.Command(lsofPath, args...)
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			continue
		}

		parsed := parseLsofOutput(out, port, opts.Protocol)
		results = append(results, parsed...)
	}

	return results, nil
}

func parseLsofOutput(out []byte, defaultPort int, defaultProto types.Protocol) []types.Process {
	var results []types.Process
	lines := strings.Split(string(out), "\n")

	var currentPID int
	var currentName string
	var currentAddr string

	flush := func() {
		if currentPID > 0 {
			name, cmdline, exe, user := GetProcessDetails(currentPID)
			if name == "" {
				name = currentName
			}
			proto := defaultProto
			if proto == types.ProtocolAll {
				proto = types.ProtocolTCP
			}

			results = append(results, types.Process{
				PID:         currentPID,
				Name:        name,
				CommandLine: cmdline,
				Exe:         exe,
				User:        user,
				Port:        defaultPort,
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

// Fallback: fuser <port>/tcp
func scanWithFuser(ports []int, opts types.FindOptions) ([]types.Process, error) {
	fuserPath, err := exec.LookPath("fuser")
	if err != nil {
		return nil, err
	}

	var results []types.Process
	for _, port := range ports {
		protocols := []string{"tcp", "udp"}
		if opts.Protocol == types.ProtocolTCP {
			protocols = []string{"tcp"}
		} else if opts.Protocol == types.ProtocolUDP {
			protocols = []string{"udp"}
		}

		for _, protoStr := range protocols {
			target := fmt.Sprintf("%d/%s", port, protoStr)
			cmd := exec.Command(fuserPath, target)
			out, _ := cmd.CombinedOutput()

			pids := strings.Fields(string(out))
			for _, pidStr := range pids {
				pidStr = strings.TrimRight(pidStr, "/cmose")
				pid, err := strconv.Atoi(pidStr)
				if err != nil || pid <= 0 {
					continue
				}

				name, cmdline, exe, user := GetProcessDetails(pid)
				results = append(results, types.Process{
					PID:         pid,
					Name:        name,
					CommandLine: cmdline,
					Exe:         exe,
					User:        user,
					Port:        port,
					Protocol:    types.Protocol(protoStr),
					State:       "LISTEN",
				})
			}
		}
	}

	return results, nil
}
