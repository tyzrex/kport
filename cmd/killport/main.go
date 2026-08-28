package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tyzrex/killport"
)

const version = "1.0.0"

// Terminal color helpers
var (
	useColors = true
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorYellow= "\033[33m"
	colorCyan  = "\033[36m"
	colorGray  = "\033[90m"
	colorBold  = "\033[1m"
)

func initColors() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		disableColors()
		return
	}
	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil || (fileInfo.Mode()&os.ModeCharDevice) == 0 {
		disableColors()
	}
}

func disableColors() {
	useColors = false
	colorReset = ""
	colorRed = ""
	colorGreen = ""
	colorYellow = ""
	colorCyan = ""
	colorGray = ""
	colorBold = ""
}

func main() {
	initColors()

	var (
		forceFlag       bool
		gracefulFlag    bool
		dryRunFlag      bool
		listFlag        bool
		jsonFlag        bool
		verboseFlag     bool
		quietFlag       bool
		interactiveFlag bool
		noColorFlag     bool
		versionFlag     bool
		protoStr        string
		timeoutStr      string
	)

	flag.BoolVar(&forceFlag, "f", false, "Force kill immediately (SIGKILL)")
	flag.BoolVar(&forceFlag, "force", false, "Force kill immediately (SIGKILL)")
	flag.BoolVar(&gracefulFlag, "g", false, "Graceful termination (SIGTERM first)")
	flag.BoolVar(&gracefulFlag, "graceful", false, "Graceful termination (SIGTERM first)")
	flag.BoolVar(&dryRunFlag, "d", false, "Dry run: preview processes without killing")
	flag.BoolVar(&dryRunFlag, "dry-run", false, "Dry run: preview processes without killing")
	flag.BoolVar(&listFlag, "l", false, "List listening processes without killing")
	flag.BoolVar(&listFlag, "list", false, "List listening processes without killing")
	flag.BoolVar(&jsonFlag, "j", false, "Output results in JSON format")
	flag.BoolVar(&jsonFlag, "json", false, "Output results in JSON format")
	flag.BoolVar(&verboseFlag, "v", false, "Verbose output showing command lines and paths")
	flag.BoolVar(&verboseFlag, "verbose", false, "Verbose output showing command lines and paths")
	flag.BoolVar(&quietFlag, "q", false, "Quiet mode (no stdout output)")
	flag.BoolVar(&quietFlag, "quiet", false, "Quiet mode (no stdout output)")
	flag.BoolVar(&interactiveFlag, "i", false, "Prompt for confirmation before killing each process")
	flag.BoolVar(&interactiveFlag, "interactive", false, "Prompt for confirmation before killing each process")
	flag.BoolVar(&noColorFlag, "no-color", false, "Disable color output")
	flag.BoolVar(&versionFlag, "version", false, "Print version")
	flag.StringVar(&protoStr, "p", "all", "Protocol filter (tcp, udp, all)")
	flag.StringVar(&protoStr, "protocol", "all", "Protocol filter (tcp, udp, all)")
	flag.StringVar(&timeoutStr, "t", "1.5s", "Graceful shutdown timeout before force kill")
	flag.StringVar(&timeoutStr, "timeout", "1.5s", "Graceful shutdown timeout before force kill")

	flag.Usage = printCustomHelp
	flag.Parse()

	if noColorFlag {
		disableColors()
	}

	if versionFlag {
		fmt.Printf("killport v%s\n", version)
		os.Exit(0)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: invalid timeout '%s': %v%s\n", colorRed, timeoutStr, err, colorReset)
		os.Exit(1)
	}

	args := flag.Args()

	// Parse ports from command line arguments
	ports, err := parsePortArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	protocol := killport.Protocol(protoStr).Normalize()

	// 1. List Mode: if -l / --list or if no ports provided and list requested
	if listFlag || (len(ports) == 0 && len(args) == 0 && !quietFlag) {
		runListMode(ports, protocol, jsonFlag, verboseFlag)
		return
	}

	if len(ports) == 0 {
		printCustomHelp()
		os.Exit(1)
	}

	// 2. Discover processes
	findOpts := killport.FindOptions{Protocol: protocol}
	procs, err := killport.FindPorts(ports, findOpts)
	if err != nil {
		if !quietFlag {
			fmt.Fprintf(os.Stderr, "%sError finding processes: %v%s\n", colorRed, err, colorReset)
		}
		os.Exit(1)
	}

	if len(procs) == 0 {
		if jsonFlag {
			fmt.Println("[]")
		} else if !quietFlag {
			fmt.Printf("%sℹ No processes found listening on port(s): %s%s\n", colorCyan, formatPortList(ports), colorReset)
		}
		os.Exit(0)
	}

	// 3. Dry-run mode
	if dryRunFlag {
		if jsonFlag {
			data, _ := json.MarshalIndent(procs, "", "  ")
			fmt.Println(string(data))
		} else if !quietFlag {
			fmt.Printf("%s[DRY RUN] Found %d process(es) listening on port(s) %s:%s\n\n", colorYellow, len(procs), formatPortList(ports), colorReset)
			for _, p := range procs {
				printProcessLine(p, verboseFlag)
			}
		}
		return
	}

	// 4. Interactive mode or Kill execution
	killOpts := killport.KillOptions{
		Protocol: protocol,
		Force:    forceFlag,
		Timeout:  timeout,
		DryRun:   false,
	}

	var results []killport.KillResult
	scanner := bufio.NewScanner(os.Stdin)
	hasError := false

	for _, p := range procs {
		if interactiveFlag {
			fmt.Printf("%s? Kill %s (PID %d) on port %d [%s]? [y/N]: %s", colorYellow, p.Name, p.PID, p.Port, strings.ToUpper(string(p.Protocol)), colorReset)
			if !scanner.Scan() {
				break
			}
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans != "y" && ans != "yes" {
				if !quietFlag {
					fmt.Printf("%s  Skipped PID %d%s\n", colorGray, p.PID, colorReset)
				}
				continue
			}
		}

		res := killport.KillProcess(p, killOpts)
		results = append(results, res)

		if res.Error != nil {
			hasError = true
		}

		if !quietFlag && !jsonFlag {
			if res.Killed {
				sigInfo := ""
				if res.Signal != "" {
					sigInfo = fmt.Sprintf(" (%s)", res.Signal)
				}
				fmt.Printf("%s✓ Killed '%s' (PID %d)%s on port %d [%s]%s\n",
					colorGreen, p.Name, p.PID, sigInfo, p.Port, strings.ToUpper(string(p.Protocol)), colorReset)
				if verboseFlag && p.CommandLine != "" {
					fmt.Printf("  %scmd: %s%s\n", colorGray, p.CommandLine, colorReset)
				}
			} else {
				fmt.Printf("%s✗ Failed to kill '%s' (PID %d) on port %d: %v%s\n",
					colorRed, p.Name, p.PID, p.Port, res.Error, colorReset)
			}
		}
	}

	if jsonFlag {
		type jsonOut struct {
			Port    int              `json:"port"`
			PID     int              `json:"pid"`
			Name    string           `json:"name"`
			Killed  bool             `json:"killed"`
			Signal  string           `json:"signal,omitempty"`
			Error   string           `json:"error,omitempty"`
			Process killport.Process `json:"process"`
		}
		var outList []jsonOut
		for _, r := range results {
			errStr := ""
			if r.Error != nil {
				errStr = r.Error.Error()
			}
			outList = append(outList, jsonOut{
				Port:    r.Port,
				PID:     r.Process.PID,
				Name:    r.Process.Name,
				Killed:  r.Killed,
				Signal:  r.Signal,
				Error:   errStr,
				Process: r.Process,
			})
		}
		data, _ := json.MarshalIndent(outList, "", "  ")
		fmt.Println(string(data))
	}

	if hasError {
		os.Exit(1)
	}
}

func runListMode(ports []int, proto killport.Protocol, jsonOut bool, verbose bool) {
	var procs []killport.Process
	var err error

	if len(ports) > 0 {
		procs, err = killport.FindPorts(ports, killport.FindOptions{Protocol: proto})
	} else {
		procs, err = killport.FindAll(killport.FindOptions{Protocol: proto})
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError listing ports: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Sort by port number
	sort.Slice(procs, func(i, j int) bool {
		if procs[i].Port == procs[j].Port {
			return procs[i].PID < procs[j].PID
		}
		return procs[i].Port < procs[j].Port
	})

	if jsonOut {
		data, _ := json.MarshalIndent(procs, "", "  ")
		fmt.Println(string(data))
		return
	}

	if len(procs) == 0 {
		if len(ports) > 0 {
			fmt.Printf("%sNo active processes found on port(s): %s%s\n", colorCyan, formatPortList(ports), colorReset)
		} else {
			fmt.Printf("%sNo active listening ports found on the system.%s\n", colorCyan, colorReset)
		}
		return
	}

	fmt.Printf("%s%-8s %-6s %-10s %-20s %-10s %s%s\n",
		colorBold, "PORT", "PROTO", "PID", "NAME", "USER", "ADDRESS", colorReset)
	fmt.Println(strings.Repeat("─", 75))

	for _, p := range procs {
		pName := p.Name
		if len(pName) > 20 {
			pName = pName[:17] + "..."
		}
		uName := p.User
		if uName == "" {
			uName = "-"
		}
		if len(uName) > 10 {
			uName = uName[:7] + "..."
		}

		fmt.Printf("%-8d %-6s %-10d %-20s %-10s %s\n",
			p.Port,
			strings.ToUpper(string(p.Protocol)),
			p.PID,
			pName,
			uName,
			p.Address,
		)
		if verbose && p.CommandLine != "" {
			fmt.Printf("  %s└─ %s%s\n", colorGray, p.CommandLine, colorReset)
		}
	}
}

func printProcessLine(p killport.Process, verbose bool) {
	fmt.Printf("  • %sPort %d%s [%s]: PID %s%d%s (%s)\n",
		colorCyan, p.Port, colorReset, strings.ToUpper(string(p.Protocol)),
		colorBold, p.PID, colorReset, p.Name)
	if p.Address != "" {
		fmt.Printf("    Address: %s\n", p.Address)
	}
	if p.User != "" {
		fmt.Printf("    User:    %s\n", p.User)
	}
	if verbose && p.CommandLine != "" {
		fmt.Printf("    Command: %s\n", p.CommandLine)
	}
}

func parsePortArgs(args []string) ([]int, error) {
	var ports []int
	seen := make(map[int]bool)

	for _, arg := range args {
		// Strip leading colons like :3000
		arg = strings.TrimPrefix(arg, ":")

		// Handle comma-separated lists like 3000,8080,9000
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(strings.TrimPrefix(part, ":"))
			if part == "" {
				continue
			}

			// Handle range format like 3000-3005
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

			// Single port
			p, err := strconv.Atoi(part)
			if err != nil || p <= 0 || p > 65535 {
				return nil, fmt.Errorf("invalid port number '%s' (must be 1-65535)", part)
			}
			if !seen[p] {
				seen[p] = true
				ports = append(ports, p)
			}
		}
	}

	return ports, nil
}

func formatPortList(ports []int) string {
	var strs []string
	for _, p := range ports {
		strs = append(strs, strconv.Itoa(p))
	}
	return strings.Join(strs, ", ")
}

func printCustomHelp() {
	help := `killport - Lightning-fast utility & library to kill processes on ports

USAGE:
    killport [OPTIONS] <PORTS...>

EXAMPLES:
    killport 3000                  Kill process listening on port 3000
    killport 3000 8080 5432        Kill processes on multiple ports
    killport :3000                 Kill port with colon prefix
    killport 3000-3005             Kill range of ports
    killport 3000,8080,9000        Kill comma-separated ports
    killport -f 3000               Force kill immediately (SIGKILL)
    killport -d 3000               Dry-run (preview processes without killing)
    killport -l                    List all listening ports and processes
    killport -l 3000 8080          Inspect specific ports without killing
    killport -i 3000 8080          Interactive confirmation before killing
    killport -j 3000               Output result as JSON

OPTIONS:
    -f, --force                    Force kill immediately (SIGKILL)
    -g, --graceful                 Graceful termination (SIGTERM first, then SIGKILL)
    -t, --timeout <dur>            Graceful shutdown timeout (default: 1.5s)
    -p, --protocol <proto>         Protocol filter: tcp, udp, all (default: all)
    -d, --dry-run                  Preview processes without killing
    -l, --list                     List listening processes
    -i, --interactive              Prompt before killing each process
    -j, --json                     Output result in JSON format
    -v, --verbose                  Show full command lines and details
    -q, --quiet                    Suppress output
        --no-color                 Disable colorized terminal output
        --version                  Print version
    -h, --help                     Show this help message
`
	fmt.Print(help)
}
