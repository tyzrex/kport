package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/completion"
	"github.com/tyzrex/kport/internal/config"
	"github.com/tyzrex/kport/internal/detect"
	"github.com/tyzrex/kport/internal/docker"
	"github.com/tyzrex/kport/internal/notify"
	"github.com/tyzrex/kport/internal/tui"
)

const version = "1.2.0"

// Terminal colors
var (
	useColors   = true
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorMagenta= "\033[35m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func initColors() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		disableColors()
		return
	}
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
	colorMagenta = ""
	colorGray = ""
	colorBold = ""
}

func isTerminal() bool {
	inInfo, err1 := os.Stdin.Stat()
	outInfo, err2 := os.Stdout.Stat()
	if err1 != nil || err2 != nil {
		return false
	}
	return (inInfo.Mode()&os.ModeCharDevice) != 0 && (outInfo.Mode()&os.ModeCharDevice) != 0
}

func main() {
	initColors()
	startTime := time.Now()

	// Load user custom configuration (~/.config/killport/config.json)
	userCfg := config.LoadConfig()
	for k, v := range userCfg.Presets {
		killport.Presets[k] = v
	}

	// Check completion subcommand before flag parsing
	if len(os.Args) >= 2 && os.Args[1] == "completion" {
		shell := "bash"
		if len(os.Args) >= 3 {
			shell = os.Args[2]
		}
		if err := completion.GenerateCompletion(shell, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		return
	}

	var (
		forceFlag       bool
		gracefulFlag    bool
		dryRunFlag      bool
		listFlag        bool
		jsonFlag        bool
		verboseFlag     bool
		quietFlag       bool
		interactiveFlag bool
		tuiFlag         bool
		watchFlag       bool
		waitFlag        bool
		treeFlag        bool
		inspectFlag     bool
		notifyFlag      bool
		benchFlag       bool
		forceProtected  bool
		noColorFlag     bool
		versionFlag     bool
		protoStr        string
		nameFilter      string
		timeoutStr      string
		watchIntStr     string
	)

	flag.BoolVar(&forceFlag, "f", false, "Force kill immediately (SIGKILL)")
	flag.BoolVar(&forceFlag, "force", false, "Force kill immediately (SIGKILL)")
	flag.BoolVar(&gracefulFlag, "g", false, "Graceful termination (SIGTERM first)")
	flag.BoolVar(&gracefulFlag, "graceful", false, "Graceful termination (SIGTERM first)")
	flag.BoolVar(&dryRunFlag, "d", false, "Dry run: preview processes without killing")
	flag.BoolVar(&dryRunFlag, "dry-run", false, "Dry run: preview processes without killing")
	flag.BoolVar(&listFlag, "l", false, "List listening processes without killing")
	flag.BoolVar(&listFlag, "list", false, "List listening processes without killing")
	flag.BoolVar(&tuiFlag, "ui", false, "Launch interactive TUI process manager")
	flag.BoolVar(&tuiFlag, "tui", false, "Launch interactive TUI process manager")
	flag.BoolVar(&jsonFlag, "j", false, "Output results in JSON format")
	flag.BoolVar(&jsonFlag, "json", false, "Output results in JSON format")
	flag.BoolVar(&verboseFlag, "v", false, "Verbose output showing command lines and paths")
	flag.BoolVar(&verboseFlag, "verbose", false, "Verbose output showing command lines and paths")
	flag.BoolVar(&quietFlag, "q", false, "Quiet mode (no stdout output)")
	flag.BoolVar(&quietFlag, "quiet", false, "Quiet mode (no stdout output)")
	flag.BoolVar(&interactiveFlag, "i", false, "Prompt for confirmation before killing each process")
	flag.BoolVar(&interactiveFlag, "interactive", false, "Prompt for confirmation before killing each process")
	flag.BoolVar(&watchFlag, "w", false, "Watch and continuously kill processes on specified ports")
	flag.BoolVar(&watchFlag, "watch", false, "Watch and continuously kill processes on specified ports")
	flag.BoolVar(&waitFlag, "W", false, "Wait until port is completely released before exiting")
	flag.BoolVar(&waitFlag, "wait", false, "Wait until port is completely released before exiting")
	flag.BoolVar(&treeFlag, "T", false, "Kill process tree (all descendant child processes)")
	flag.BoolVar(&treeFlag, "tree", false, "Kill process tree (all descendant child processes)")
	flag.BoolVar(&inspectFlag, "inspect", false, "Show detailed process inspection (Memory, CWD, PPID, Tree)")
	flag.BoolVar(&notifyFlag, "N", false, "Send native desktop notification on kill")
	flag.BoolVar(&notifyFlag, "notify", false, "Send native desktop notification on kill")
	flag.BoolVar(&benchFlag, "bench", false, "Show execution latency benchmark")
	flag.BoolVar(&forceProtected, "force-protected", false, "Allow killing critical protected system ports (e.g. SSH 22)")
	flag.BoolVar(&noColorFlag, "no-color", false, "Disable color output")
	flag.BoolVar(&versionFlag, "version", false, "Print version")
	flag.StringVar(&protoStr, "p", "all", "Protocol filter (tcp, udp, all)")
	flag.StringVar(&protoStr, "protocol", "all", "Protocol filter (tcp, udp, all)")
	flag.StringVar(&nameFilter, "n", "", "Filter processes by name or command line substring")
	flag.StringVar(&nameFilter, "name", "", "Filter processes by name or command line substring")
	flag.StringVar(&timeoutStr, "t", "1.5s", "Graceful shutdown timeout before force kill")
	flag.StringVar(&timeoutStr, "timeout", "1.5s", "Graceful shutdown timeout before force kill")
	flag.StringVar(&watchIntStr, "interval", "500ms", "Polling interval for watch mode")

	flag.Usage = printCustomHelp
	if err := flag.CommandLine.Parse(normalizeArgs(os.Args[1:])); err != nil {
		os.Exit(1)
	}

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

	watchInterval, err := time.ParseDuration(watchIntStr)
	if err != nil {
		watchInterval = 500 * time.Millisecond
	}

	args := flag.Args()
	protocol := killport.Protocol(protoStr).Normalize()

	// Special Subcommands: 'free' / 'suggest'
	if len(args) >= 1 && (args[0] == "free" || args[0] == "suggest") {
		runFreeCommand(args, protocol)
		return
	}

	// 1. Interactive TUI Mode: if `kport` run without args in interactive terminal, or if -ui / --tui flag
	if (len(args) == 0 && isTerminal() && !listFlag && !jsonFlag && !quietFlag) || tuiFlag {
		runTUIMode(protocol, nameFilter)
		return
	}

	// 2. Project & CWD auto-detection (e.g. `kport .` or `kport auto` or `kport cwd` / `kport here`)
	if len(args) == 1 && (args[0] == "." || args[0] == "auto" || args[0] == "cwd" || args[0] == "here") {
		runAutoProjectKill(protocol, forceFlag, timeout, treeFlag, waitFlag, dryRunFlag, quietFlag, jsonFlag)
		return
	}

	// 3. List Mode
	if listFlag || (len(args) == 0 && !quietFlag) {
		ports, _ := parsePortArgs(args)
		runListMode(ports, protocol, nameFilter, jsonFlag, verboseFlag || inspectFlag)
		return
	}

	// Parse ports, ranges, and presets
	ports, err := parsePortArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	if len(ports) == 0 {
		printCustomHelp()
		os.Exit(1)
	}

	// Protected Ports check
	if !forceProtected && !forceFlag {
		for _, port := range ports {
			if desc, isProt := killport.IsProtectedPort(port); isProt {
				if !interactivePrompt(fmt.Sprintf("Port %d is a critical system service (%s). Are you sure you want to target it? [y/N]: ", port, desc)) {
					fmt.Fprintf(os.Stderr, "%sAborted targeting protected port %d%s\n", colorYellow, port, colorReset)
					os.Exit(1)
				}
			}
		}
	}

	// Watch Mode
	if watchFlag {
		runWatchMode(ports, protocol, forceFlag, timeout, treeFlag, watchInterval, quietFlag, jsonFlag, notifyFlag)
		return
	}

	// Discover processes
	findOpts := killport.FindOptions{
		Protocol:    protocol,
		NameFilter:  nameFilter,
		IncludeTree: treeFlag || inspectFlag,
	}
	procs, err := killport.FindPorts(ports, findOpts)
	if err != nil {
		if !quietFlag {
			fmt.Fprintf(os.Stderr, "%sError finding processes: %v%s\n", colorRed, err, colorReset)
		}
		os.Exit(1)
	}

	// Check Docker container mappings for docker-proxy processes
	for i := range procs {
		if strings.Contains(procs[i].Name, "docker") || strings.Contains(procs[i].CommandLine, "docker-proxy") {
			if cInfo, ok := docker.GetContainerByPort(procs[i].Port); ok {
				procs[i].Name = fmt.Sprintf("docker:%s", cInfo.Name)
			}
		}
	}

	if len(procs) == 0 {
		if jsonFlag {
			fmt.Println("[]")
		} else if !quietFlag {
			fmt.Printf("%sℹ No matching processes found listening on port(s): %s%s\n", colorCyan, formatPortList(ports), colorReset)
		}
		os.Exit(0)
	}

	// Dry-run or Inspect mode
	if dryRunFlag || inspectFlag {
		if jsonFlag {
			data, _ := json.MarshalIndent(procs, "", "  ")
			fmt.Println(string(data))
		} else if !quietFlag {
			header := "[DRY RUN]"
			if inspectFlag && !dryRunFlag {
				header = "[INSPECT]"
			}
			fmt.Printf("%s%s Found %d process(es) on port(s) %s:%s\n\n", colorYellow, header, len(procs), formatPortList(ports), colorReset)
			for _, p := range procs {
				printProcessInspection(p, verboseFlag || inspectFlag)
			}
		}
		if dryRunFlag {
			return
		}
	}

	// Kill execution
	killOpts := killport.KillOptions{
		Protocol:    protocol,
		Force:       forceFlag,
		Timeout:     timeout,
		KillTree:    treeFlag,
		WaitForFree: waitFlag,
		DryRun:      false,
	}

	var results []killport.KillResult
	scanner := bufio.NewScanner(os.Stdin)
	hasError := false

	for _, p := range procs {
		if interactiveFlag {
			promptMsg := fmt.Sprintf("Kill %s (PID %d) on port %d [%s]?", p.Name, p.PID, p.Port, strings.ToUpper(string(p.Protocol)))
			if treeFlag && len(p.ChildrenPIDs) > 0 {
				promptMsg += fmt.Sprintf(" (and %d child processes)", len(p.ChildrenPIDs))
			}
			fmt.Printf("%s? %s [y/N]: %s", colorYellow, promptMsg, colorReset)
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
				treeInfo := ""
				if res.ChildrenKilled > 0 {
					treeInfo = fmt.Sprintf(" [+%d child processes]", res.ChildrenKilled)
				}
				fmt.Printf("%s✓ Killed '%s' (PID %d)%s%s on port %d [%s]%s\n",
					colorGreen, p.Name, p.PID, sigInfo, treeInfo, p.Port, strings.ToUpper(string(p.Protocol)), colorReset)
				if verboseFlag && p.CommandLine != "" {
					fmt.Printf("  %scmd: %s%s\n", colorGray, p.CommandLine, colorReset)
				}
			} else {
				fmt.Printf("%s✗ Failed to kill '%s' (PID %d) on port %d: %v%s\n",
					colorRed, p.Name, p.PID, p.Port, res.Error, colorReset)
			}
		}

		if notifyFlag && res.Killed {
			notify.SendNotification("killport", fmt.Sprintf("Killed '%s' (PID %d) on port %d", p.Name, p.PID, p.Port))
		}
	}

	// Verify wait release
	if waitFlag && !quietFlag && !jsonFlag {
		fmt.Printf("%s⏳ Verifying port(s) are released by the OS...%s\n", colorGray, colorReset)
		for _, port := range ports {
			if err := killport.WaitForFree(port, 3*time.Second, protocol); err == nil {
				fmt.Printf("%s✓ Port %d is fully free!%s\n", colorGreen, port, colorReset)
			}
		}
	}

	if benchFlag && !quietFlag && !jsonFlag {
		fmt.Printf("%s⚡ Completed in %v%s\n", colorCyan, time.Since(startTime).Round(time.Microsecond), colorReset)
	}

	if jsonFlag {
		type jsonOut struct {
			Port           int              `json:"port"`
			PID            int              `json:"pid"`
			Name           string           `json:"name"`
			Killed         bool             `json:"killed"`
			Signal         string           `json:"signal,omitempty"`
			ChildrenKilled int              `json:"children_killed,omitempty"`
			Error          string           `json:"error,omitempty"`
			Process        killport.Process `json:"process"`
		}
		var outList []jsonOut
		for _, r := range results {
			errStr := ""
			if r.Error != nil {
				errStr = r.Error.Error()
			}
			outList = append(outList, jsonOut{
				Port:           r.Port,
				PID:            r.Process.PID,
				Name:           r.Process.Name,
				Killed:         r.Killed,
				Signal:         r.Signal,
				ChildrenKilled: r.ChildrenKilled,
				Error:          errStr,
				Process:        r.Process,
			})
		}
		data, _ := json.MarshalIndent(outList, "", "  ")
		fmt.Println(string(data))
	}

	if hasError {
		os.Exit(1)
	}
}

func runTUIMode(proto killport.Protocol, nameFilter string) {
	procs, err := killport.FindAll(killport.FindOptions{
		Protocol:   proto,
		NameFilter: nameFilter,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		return
	}

	if len(procs) == 0 {
		fmt.Printf("%sℹ No active listening ports found on the system.%s\n", colorCyan, colorReset)
		return
	}

	toKill, force, canceled := tui.RunTUI(procs)
	if canceled || len(toKill) == 0 {
		return
	}

	opts := killport.KillOptions{
		Force:   force,
		Timeout: 1500 * time.Millisecond,
	}

	fmt.Printf("\n%s==> Terminating %d process(es)...%s\n\n", colorBold, len(toKill), colorReset)
	for _, p := range toKill {
		res := killport.KillProcess(p, opts)
		if res.Killed {
			sig := res.Signal
			if sig == "" {
				sig = "OK"
			}
			fmt.Printf("%s✓ Killed '%s' (PID %d) (%s) on port %d [%s]%s\n",
				colorGreen, p.Name, p.PID, sig, p.Port, strings.ToUpper(string(p.Protocol)), colorReset)
		} else {
			fmt.Printf("%s✗ Failed to kill '%s' (PID %d) on port %d: %v%s\n",
				colorRed, p.Name, p.PID, p.Port, res.Error, colorReset)
		}
	}
}

func runAutoProjectKill(proto killport.Protocol, force bool, timeout time.Duration, tree bool, wait bool, dryRun bool, quiet bool, jsonOut bool) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError getting current directory: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	allProcs, err := killport.FindAll(killport.FindOptions{Protocol: proto, IncludeTree: tree})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError discovering processes: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// 1. Look for processes whose CWD matches current directory
	cwdProcs := detect.FindProcessesByCWD(cwd, allProcs)

	// 2. Look for project framework configs (e.g. Next.js, Vite, docker-compose)
	var suspectedPorts []int
	var frameworkName string
	if pInfo, ok := detect.DetectProject(cwd); ok {
		suspectedPorts = pInfo.Ports
		frameworkName = pInfo.Framework
	}

	targetProcsMap := make(map[int]killport.Process)
	for _, p := range cwdProcs {
		targetProcsMap[p.PID] = p
	}

	for _, p := range allProcs {
		for _, sp := range suspectedPorts {
			if p.Port == sp {
				targetProcsMap[p.PID] = p
			}
		}
	}

	var targetProcs []killport.Process
	for _, p := range targetProcsMap {
		targetProcs = append(targetProcs, p)
	}

	if len(targetProcs) == 0 {
		if !quiet && !jsonOut {
			fmt.Printf("%sℹ No running dev servers or ports found for project in %s%s\n", colorCyan, cwd, colorReset)
			if frameworkName != "" {
				fmt.Printf("  (Detected %s on port(s) %v, but they are not currently running)\n", frameworkName, suspectedPorts)
			}
		}
		return
	}

	if !quiet && !jsonOut {
		detectedMsg := ""
		if frameworkName != "" {
			detectedMsg = fmt.Sprintf(" [%s Project]", frameworkName)
		}
		fmt.Printf("%s⚡ Detected %d process(es) for current directory%s:%s\n\n",
			colorBold+colorCyan, len(targetProcs), detectedMsg, colorReset)
		for _, p := range targetProcs {
			printProcessLine(p, true)
		}
		fmt.Println()
	}

	if dryRun {
		return
	}

	killOpts := killport.KillOptions{
		Protocol:    proto,
		Force:       force,
		Timeout:     timeout,
		KillTree:    tree,
		WaitForFree: wait,
	}

	for _, p := range targetProcs {
		res := killport.KillProcess(p, killOpts)
		if !quiet && !jsonOut {
			if res.Killed {
				fmt.Printf("%s✓ Killed '%s' (PID %d) on port %d%s\n", colorGreen, p.Name, p.PID, p.Port, colorReset)
			} else {
				fmt.Printf("%s✗ Failed to kill PID %d on port %d: %v%s\n", colorRed, p.PID, p.Port, res.Error, colorReset)
			}
		}
	}
}

func runFreeCommand(args []string, proto killport.Protocol) {
	startPort := 3000
	endPort := 4000
	count := 5

	if len(args) >= 2 {
		arg := strings.TrimPrefix(args[1], ":")
		if strings.Contains(arg, "-") {
			parts := strings.Split(arg, "-")
			if len(parts) == 2 {
				s, err1 := strconv.Atoi(parts[0])
				e, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					startPort = s
					endPort = e
					count = endPort - startPort + 1
				}
			}
		} else if p, err := strconv.Atoi(arg); err == nil {
			startPort = p
			endPort = p + 50
			count = 5
		}
	}

	ports, err := killport.FindFreePorts(startPort, endPort, count, proto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Available Free Port(s):%s\n", colorGreen+colorBold, colorReset)
	for _, p := range ports {
		fmt.Printf("  • %sPort %d%s [FREE]\n", colorCyan, p, colorReset)
	}
}

func runWatchMode(ports []int, proto killport.Protocol, force bool, timeout time.Duration, tree bool, interval time.Duration, quiet bool, jsonOut bool, notifyUser bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if !quiet && !jsonOut {
			fmt.Printf("\n%sStopping watch mode.%s\n", colorCyan, colorReset)
		}
		cancel()
	}()

	if !quiet && !jsonOut {
		fmt.Printf("%s👀 Watching port(s) %s every %v (Press Ctrl+C to stop)...%s\n",
			colorCyan, formatPortList(ports), interval, colorReset)
	}

	opts := killport.KillOptions{
		Protocol: proto,
		Force:    force,
		Timeout:  timeout,
		KillTree: tree,
	}

	_ = killport.Watch(ctx, ports, opts, interval, func(res killport.KillResult) {
		if !quiet && !jsonOut {
			fmt.Printf("%s⚡ [WATCH] Killed '%s' (PID %d) on port %d [%s]%s\n",
				colorGreen, res.Process.Name, res.Process.PID, res.Port, strings.ToUpper(string(res.Process.Protocol)), colorReset)
		} else if jsonOut {
			data, _ := json.Marshal(res)
			fmt.Println(string(data))
		}
		if notifyUser && res.Killed {
			notify.SendNotification("killport", fmt.Sprintf("Auto-killed '%s' (PID %d) on port %d", res.Process.Name, res.Process.PID, res.Port))
		}
	})
}

func runListMode(ports []int, proto killport.Protocol, nameFilter string, jsonOut bool, verbose bool) {
	findOpts := killport.FindOptions{
		Protocol:    proto,
		NameFilter:  nameFilter,
		IncludeTree: verbose,
	}

	var procs []killport.Process
	var err error

	if len(ports) > 0 {
		procs, err = killport.FindPorts(ports, findOpts)
	} else {
		procs, err = killport.FindAll(findOpts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError listing ports: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	for i := range procs {
		if strings.Contains(procs[i].Name, "docker") || strings.Contains(procs[i].CommandLine, "docker-proxy") {
			if cInfo, ok := docker.GetContainerByPort(procs[i].Port); ok {
				procs[i].Name = fmt.Sprintf("docker:%s", cInfo.Name)
			}
		}
	}

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

	fmt.Printf("%s%-8s %-6s %-10s %-20s %-10s %-10s %s%s\n",
		colorBold, "PORT", "PROTO", "PID", "NAME", "MEMORY", "USER", "ADDRESS", colorReset)
	fmt.Println(strings.Repeat("─", 85))

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
		mem := p.MemoryHuman
		if mem == "" {
			mem = "-"
		}

		fmt.Printf("%-8d %-6s %-10d %-20s %-10s %-10s %s\n",
			p.Port,
			strings.ToUpper(string(p.Protocol)),
			p.PID,
			pName,
			mem,
			uName,
			p.Address,
		)
		if verbose {
			if p.CommandLine != "" {
				fmt.Printf("  %s├─ Cmd:  %s%s\n", colorGray, p.CommandLine, colorReset)
			}
			if p.Exe != "" {
				fmt.Printf("  %s├─ Exe:  %s%s\n", colorGray, p.Exe, colorReset)
			}
			if p.Cwd != "" {
				fmt.Printf("  %s├─ Cwd:  %s%s\n", colorGray, p.Cwd, colorReset)
			}
			if len(p.ChildrenPIDs) > 0 {
				fmt.Printf("  %s└─ Tree: %d child PID(s) %v%s\n", colorGray, len(p.ChildrenPIDs), p.ChildrenPIDs, colorReset)
			}
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
	if p.Cwd != "" {
		fmt.Printf("    CWD:     %s\n", p.Cwd)
	}
	if verbose && p.CommandLine != "" {
		fmt.Printf("    Command: %s\n", p.CommandLine)
	}
}

func printProcessInspection(p killport.Process, verbose bool) {
	fmt.Printf("  • %sPort %d%s [%s]: PID %s%d%s (%s)\n",
		colorCyan, p.Port, colorReset, strings.ToUpper(string(p.Protocol)),
		colorBold, p.PID, colorReset, p.Name)
	if p.Address != "" {
		fmt.Printf("    Address: %s\n", p.Address)
	}
	if p.User != "" {
		fmt.Printf("    User:    %s\n", p.User)
	}
	if p.PPID > 0 {
		fmt.Printf("    PPID:    %d\n", p.PPID)
	}
	if p.MemoryHuman != "" {
		fmt.Printf("    Memory:  %s\n", p.MemoryHuman)
	}
	if p.Cwd != "" {
		fmt.Printf("    CWD:     %s\n", p.Cwd)
	}
	if p.Exe != "" {
		fmt.Printf("    Exe:     %s\n", p.Exe)
	}
	if p.CommandLine != "" {
		fmt.Printf("    Command: %s\n", p.CommandLine)
	}
	if len(p.ChildrenPIDs) > 0 {
		fmt.Printf("    Tree:    %d child PID(s): %v\n", len(p.ChildrenPIDs), p.ChildrenPIDs)
	}
	fmt.Println()
}

func interactivePrompt(msg string) bool {
	fmt.Print(colorYellow, msg, colorReset)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "y" || ans == "yes"
}

func parsePortArgs(args []string) ([]int, error) {
	var ports []int
	seen := make(map[int]bool)

	for _, arg := range args {
		arg = strings.TrimPrefix(arg, ":")

		if presetPorts, ok := killport.ResolvePreset(arg); ok {
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

			if presetPorts, ok := killport.ResolvePreset(part); ok {
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
    killport [OPTIONS] <PORTS | PRESETS | COMMAND...>
    kport [OPTIONS] <PORTS | PRESETS | COMMAND...>

COMMANDS:
    (no args)                      Launch interactive TUI dashboard (when run in a terminal)
    killport . / auto / here       Auto-detect and kill dev servers for the current project/CWD
    killport free <port|range>     Find and suggest next available free ports
    killport completion <shell>    Generate autocompletion script (bash, zsh, fish, powershell)

EXAMPLES:
    killport 3000                  Kill process listening on port 3000
    killport 3000 8080 5432        Kill processes on multiple ports
    killport :3000                 Kill port with colon prefix
    killport 3000-3005             Kill range of ports
    killport 3000,8080,9000        Kill comma-separated ports
    killport next                  Kill preset ports for Next.js (port 3000)
    killport vite                  Kill preset ports for Vite (port 5173)
    killport dev                   Kill common web dev ports (3000, 5173, 8080, ...)
    killport postgres              Kill PostgreSQL port (5432)
    killport redis                 Kill Redis port (6379)
    killport -f 3000               Force kill immediately (SIGKILL)
    killport -T 3000               Kill process tree (parent and all child processes)
    killport -W 3000               Kill and wait until port is completely released
    killport -w 3000               Watch mode: continuously kill anything binding to 3000
    killport -d 3000               Dry-run (preview processes without killing)
    killport --inspect 3000        Inspect detailed process info (Memory, CWD, Tree)
    killport --ui                  Launch interactive TUI process manager
    killport -l                    List all listening ports and processes
    killport -l -n node            List only processes matching "node"
    killport -i 3000 8080          Interactive confirmation before killing
    killport -j 3000               Output result as JSON
    killport free 3000             Suggest next available free port

PRESETS:
    web, dev, db, next, react, remix, vite, astro, svelte, vue, angular,
    django, fastapi, flask, rails, redis, postgres, mysql, mariadb, mongo

OPTIONS:
    -f, --force                    Force kill immediately (SIGKILL)
    -g, --graceful                 Graceful termination (SIGTERM first, then SIGKILL)
    -t, --timeout <dur>            Graceful shutdown timeout (default: 1.5s)
    -T, --tree                     Kill process tree (descendant child processes)
    -W, --wait                     Wait until port is completely released before exiting
    -w, --watch                    Watch mode: continuously kill processes on ports
    -n, --name <str>               Filter processes by name or command substring
    -p, --protocol <proto>         Protocol filter: tcp, udp, all (default: all)
    -d, --dry-run                  Preview processes without killing
        --inspect                  Show detailed process inspection (Memory, CWD, Tree)
        --ui, --tui                Launch interactive TUI manager
    -l, --list                     List listening processes
    -i, --interactive              Prompt before killing each process
    -N, --notify                   Send native OS desktop notification on kill
        --bench                    Show execution latency benchmark
    -j, --json                     Output result in JSON format
    -v, --verbose                  Show full command lines and details
    -q, --quiet                    Suppress output
        --force-protected          Allow killing protected ports (e.g. SSH 22)
        --no-color                 Disable colorized terminal output
        --version                  Print version
    -h, --help                     Show this help message
`
	fmt.Print(help)
}
