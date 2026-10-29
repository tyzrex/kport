package cli

import (
	"fmt"
	"github.com/tyzrex/kport"
	"strconv"
	"strings"
)

// normalizeArgs rearranges command-line arguments so that flags (e.g. -f, -d, -p tcp)
// can be specified anywhere (before, after, or mixed with positional ports/presets).
func normalizeArgs(args []string) []string {
	takesArg := map[string]bool{
		"-p":             true,
		"--protocol":     true,
		"-t":             true,
		"--timeout":      true,
		"-n":             true,
		"--name":         true,
		"--interval":     true,
		"--wait-timeout": true, "--user": true, "--project": true, "--exposure": true, "--sort": true, "--pid": true,
	}

	var flags []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return append(append(flags, "--"), append(positional, args[i+1:]...)...)
		}

		// Special case: completion command
		if i == 0 && arg == "completion" {
			return args
		}

		if strings.HasPrefix(arg, "-") {
			// If it's a range like "3000-3005", it's not a flag
			if !strings.HasPrefix(arg, "--") && strings.Count(arg, "-") == 1 && len(arg) > 1 && isNumericChar(arg[1]) {
				positional = append(positional, arg)
				continue
			}

			flags = append(flags, arg)
			// Check if flag takes a separate value (e.g. -p tcp)
			if takesArg[arg] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}

	return append(flags, positional...)
}

func isNumericChar(c byte) bool {
	return c >= '0' && c <= '9'
}

func parsePortArgs(args []string) ([]int, error) {
	return parsePortArgsWithResolver(args, killport.ResolvePreset)
}
func parsePortArgsWithResolver(args []string, resolve func(string) ([]int, bool)) ([]int, error) {
	var ports []int
	seen := make(map[int]bool)

	for _, arg := range args {
		arg = strings.TrimPrefix(arg, ":")

		if presetPorts, ok := resolve(arg); ok {
			for _, p := range presetPorts {
				if !seen[p] {
					seen[p] = true
					ports = append(ports, p)
				}
			}
			continue
		}

		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(strings.TrimPrefix(part, ":"))
			if part == "" {
				continue
			}

			if presetPorts, ok := resolve(part); ok {
				for _, p := range presetPorts {
					if !seen[p] {
						seen[p] = true
						ports = append(ports, p)
					}
				}
				continue
			}

			if strings.Contains(part, "-") {
				rangeParts := strings.Split(part, "-")
				if len(rangeParts) != 2 {
					return nil, fmt.Errorf("invalid port range: '%s'", part)
				}
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 != nil || err2 != nil || start <= 0 || end > 65535 || start > end {
					return nil, fmt.Errorf("invalid port range: '%s'", part)
				}
				for p := start; p <= end; p++ {
					if !seen[p] {
						seen[p] = true
						ports = append(ports, p)
					}
				}
				continue
			}

			p, err := strconv.Atoi(part)
			if err != nil || p <= 0 || p > 65535 {
				return nil, fmt.Errorf("invalid port or unknown preset '%s' (must be 1-65535 or a valid preset name)", part)
			}
			if !seen[p] {
				seen[p] = true
				ports = append(ports, p)
			}
		}
	}

	return ports, nil
}
