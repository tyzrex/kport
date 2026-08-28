package main

import "strings"

// normalizeArgs rearranges command-line arguments so that flags (e.g. -f, -d, -p tcp)
// can be specified anywhere (before, after, or mixed with positional ports/presets).
func normalizeArgs(args []string) []string {
	takesArg := map[string]bool{
		"-p":         true,
		"--protocol": true,
		"-t":         true,
		"--timeout":  true,
		"-n":         true,
		"--name":     true,
		"--interval": true,
	}

	var flags []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

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
