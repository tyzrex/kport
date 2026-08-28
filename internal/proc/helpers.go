package proc

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/user"
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
	// /proc/net/tcp stores IPv4 addresses in little-endian order
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
	// /proc/net/tcp6 stores 4 32-bit integers in little-endian order
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

// GetProcessDetails tries to obtain process name, cmdline, exe and username from OS files or /proc.
func GetProcessDetails(pid int) (name string, cmdline string, exe string, userName string) {
	if pid <= 0 {
		return "", "", "", ""
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
			// extract basename
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

	// 4. Status / UID
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
				break
			}
		}
	}

	return name, cmdline, exe, userName
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
