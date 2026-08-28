package proc

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tyzrex/killport/types"
)

// MatchPort returns true if targetPort is in the ports slice, or if ports slice is empty.
func MatchPort(ports []int, targetPort int) bool {
	if len(ports) == 0 {
		return true
	}
	for _, p := range ports {
		if p == targetPort {
			return true
		}
	}
	return false
}

// MatchesNameFilter checks if process name, exe, or cmdline matches the filter string.
func MatchesNameFilter(p types.Process, filter string) bool {
	if filter == "" {
		return true
	}
	f := strings.ToLower(filter)
	if strings.Contains(strings.ToLower(p.Name), f) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Exe), f) {
		return true
	}
	if strings.Contains(strings.ToLower(p.CommandLine), f) {
		return true
	}
	return false
}

// ParseHexPort parses a hexadecimal port string from /proc/net/tcp.
func ParseHexPort(hexStr string) (int, error) {
	val, err := strconv.ParseInt(hexStr, 16, 32)
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

// ParseHexIPv4 parses a little-endian 8-char hex string to an IPv4 string.
func ParseHexIPv4(hexStr string) string {
	if len(hexStr) != 8 {
		return hexStr
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil || len(b) != 4 {
		return hexStr
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[3], b[2], b[1], b[0])
}

// ParseHexIPv6 parses a 32-char hex string to an IPv6 string.
func ParseHexIPv6(hexStr string) string {
	if len(hexStr) != 32 {
		return hexStr
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil || len(b) != 16 {
		return hexStr
	}
	var ipBytes [16]byte
	for i := 0; i < 4; i++ {
		offset := i * 4
		ipBytes[offset+0] = b[offset+3]
		ipBytes[offset+1] = b[offset+2]
		ipBytes[offset+2] = b[offset+1]
		ipBytes[offset+3] = b[offset+0]
	}
	ip := net.IP(ipBytes[:])
	return ip.String()
}

// FormatBytes formats byte counts into human-readable representation (e.g. "24.5 MB").
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetProcessDetails populates rich process details from /proc (Linux) or OS introspection.
func GetProcessDetails(pid int) (name string, cmdline string, exe string, userName string, ppid int, cwd string, rss uint64) {
	if pid <= 0 {
		return "", "", "", "", 0, "", 0
	}

	// 1. Comm (short name)
	if commBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
		name = strings.TrimSpace(string(commBytes))
	}

	// 2. Cmdline (full command arguments)
	if cmdBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		parts := strings.Split(string(cmdBytes), "\x00")
		var validParts []string
		for _, p := range parts {
			if p != "" {
				validParts = append(validParts, p)
			}
		}
		cmdline = strings.Join(validParts, " ")
		if name == "" && len(validParts) > 0 {
			name = validParts[0]
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
		}
	}

	// 3. Exe symlink
	if exeLink, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
		exe = exeLink
		if name == "" {
			if idx := strings.LastIndex(exe, "/"); idx != -1 {
				name = exe[idx+1:]
			} else {
				name = exe
			}
		}
	}

	// 4. Cwd symlink
	if cwdLink, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
		cwd = cwdLink
	}

	// 5. Status: UID, PPid, VmRSS
	if statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		lines := strings.Split(string(statusBytes), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					uidStr := fields[1]
					if u, err := user.LookupId(uidStr); err == nil {
						userName = u.Username
					} else {
						userName = uidStr
					}
				}
			} else if strings.HasPrefix(line, "PPid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					ppid, _ = strconv.Atoi(fields[1])
				}
			} else if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.ParseUint(fields[1], 10, 64)
					rss = kb * 1024
				}
			}
		}
	}

	return name, cmdline, exe, userName, ppid, cwd, rss
}

// FindProcessChildren returns all descendant child PIDs of a given parent PID.
func FindProcessChildren(parentPID int) []int {
	if parentPID <= 0 {
		return nil
	}

	// 1. Try reading /proc/<pid>/task/<pid>/children (Linux direct kernel interface)
	taskChildrenPath := fmt.Sprintf("/proc/%d/task/%d/children", parentPID, parentPID)
	if data, err := os.ReadFile(taskChildrenPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			var directChildren []int
			for _, f := range fields {
				if cpid, err := strconv.Atoi(f); err == nil && cpid > 0 {
					directChildren = append(directChildren, cpid)
				}
			}
			var allChildren []int
			for _, cpid := range directChildren {
				allChildren = append(allChildren, cpid)
				allChildren = append(allChildren, FindProcessChildren(cpid)...)
			}
			return allChildren
		}
	}

	// 2. Fallback: scan /proc directory for PPID
	procDir, err := os.Open("/proc")
	if err != nil {
		return nil
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil
	}

	var directChildren []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry)
		if err != nil || pid <= 0 || pid == parentPID {
			continue
		}

		statusPath := filepath.Join("/proc", entry, "status")
		if statusBytes, err := os.ReadFile(statusPath); err == nil {
			lines := strings.Split(string(statusBytes), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "PPid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if p, _ := strconv.Atoi(fields[1]); p == parentPID {
							directChildren = append(directChildren, pid)
						}
					}
					break
				}
			}
		}
	}

	var allChildren []int
	for _, cpid := range directChildren {
		allChildren = append(allChildren, cpid)
		allChildren = append(allChildren, FindProcessChildren(cpid)...)
	}

	return allChildren
}

// DeduplicateProcesses removes duplicate processes matching same PID, Port and Protocol.
func DeduplicateProcesses(procs []types.Process) []types.Process {
	type key struct {
		pid      int
		port     int
		protocol types.Protocol
	}
	seen := make(map[key]bool)
	var unique []types.Process

	for _, p := range procs {
		k := key{pid: p.PID, port: p.Port, protocol: p.Protocol}
		if !seen[k] {
			seen[k] = true
			unique = append(unique, p)
		}
	}
	return unique
}
