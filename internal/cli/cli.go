// Package cli implements both the kport and killport commands.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/tyzrex/kport/internal/app"
	"github.com/tyzrex/kport/internal/completion"
	"github.com/tyzrex/kport/internal/config"
	"github.com/tyzrex/kport/internal/detect"
	"github.com/tyzrex/kport/internal/notify"
	"github.com/tyzrex/kport/internal/tui"
	"golang.org/x/term"
)

type options struct {
	service                                                                              *app.Service
	ctx                                                                                  context.Context
	pid                                                                                  int
	startID                                                                              string
	force, graceful, dry, list, json, verbose, quiet, confirm, ui                        bool
	watch, observe, wait, tree, inspect, notify, bench, allowProtected, noColor, version bool
	protocol, name, user, project, exposure, sortBy                                      string
	timeout, interval, waitTimeout                                                       time.Duration
}

func Run(version string) {
	if err := run(os.Args[1:], version); err != nil {
		fmt.Fprintln(os.Stderr, "kport:", err)
		os.Exit(1)
	}
}

func run(args []string, version string) error {
	if len(args) > 0 && args[0] == "completion" {
		shell := "bash"
		if len(args) > 1 {
			shell = args[1]
		}
		return completion.GenerateCompletion(shell, os.Stdout)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	o := options{service: app.New(cfg), ctx: ctx, force: cfg.DefaultForce, tree: cfg.DefaultTree, timeout: cfg.Timeout}
	fs := flag.NewFlagSet("kport", flag.ContinueOnError)
	boolFlag := func(p *bool, value bool, help string, names ...string) {
		for _, n := range names {
			fs.BoolVar(p, n, value, help)
		}
	}
	strFlag := func(p *string, value, help string, names ...string) {
		for _, n := range names {
			fs.StringVar(p, n, value, help)
		}
	}
	durationFlag := func(p *time.Duration, value time.Duration, help string, names ...string) {
		for _, n := range names {
			fs.DurationVar(p, n, value, help)
		}
	}
	boolFlag(&o.force, cfg.DefaultForce, "Force termination", "f", "force")
	boolFlag(&o.graceful, false, "Graceful termination, overriding default_force", "g", "graceful")
	boolFlag(&o.dry, false, "Preview without stopping anything", "d", "dry-run")
	boolFlag(&o.list, false, "List listening processes", "l", "list")
	boolFlag(&o.json, false, "Output JSON (JSON Lines for watch)", "j", "json")
	boolFlag(&o.verbose, false, "Include detailed process information", "v", "verbose")
	boolFlag(&o.quiet, false, "Suppress stdout", "q", "quiet")
	boolFlag(&o.confirm, false, "Confirm each target", "i", "interactive")
	boolFlag(&o.ui, false, "Open persistent dashboard", "ui", "tui")
	boolFlag(&o.watch, false, "Continuously stop matching processes", "w", "watch")
	boolFlag(&o.observe, false, "Observe occupancy changes without stopping processes", "observe")
	boolFlag(&o.wait, false, "Verify ports are free; fail on timeout", "W", "wait")
	boolFlag(&o.tree, cfg.DefaultTree, "Stop descendants too", "T", "tree")
	boolFlag(&o.inspect, false, "Inspect without stopping processes", "inspect")
	boolFlag(&o.notify, false, "Send desktop notifications", "N", "notify")
	boolFlag(&o.bench, false, "Print execution duration", "bench")
	boolFlag(&o.allowProtected, false, "Explicitly allow protected ports", "force-protected")
	boolFlag(&o.noColor, false, "Disable terminal color", "no-color")
	boolFlag(&o.version, false, "Print version", "version")
	strFlag(&o.protocol, "all", "tcp, udp, or all", "p", "protocol")
	strFlag(&o.name, "", "Name/command substring", "n", "name")
	strFlag(&o.user, "", "Restrict to an exact user name", "user")
	strFlag(&o.project, "", "Restrict to a project directory and its descendants", "project")
	strFlag(&o.exposure, "all", "all, localhost, public (wildcard), or interface", "exposure")
	strFlag(&o.sortBy, "port", "port, memory, project, user, name, or protocol", "sort")
	durationFlag(&o.timeout, cfg.Timeout, "Graceful timeout", "t", "timeout")
	durationFlag(&o.waitTimeout, 5*time.Second, "Port release timeout", "wait-timeout")
	durationFlag(&o.interval, time.Second, "Watch/refresh interval", "interval")
	fs.IntVar(&o.pid, "pid", 0, "Restrict to this PID and its current start identity")
	fs.Usage = func() { printCustomHelp(); fs.PrintDefaults() }
	if err = fs.Parse(normalizeArgs(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	o.noColor = o.noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	if o.version {
		fmt.Printf("kport v%s\n", version)
		return nil
	}
	if o.timeout <= 0 || o.waitTimeout <= 0 || o.interval <= 0 {
		return fmt.Errorf("timeout, wait-timeout, and interval must be positive")
	}
	switch strings.ToLower(o.protocol) {
	case "tcp", "udp", "all", "both":
		o.protocol = string(killport.Protocol(strings.ToLower(o.protocol)).Normalize())
	default:
		return fmt.Errorf("unknown protocol %q", o.protocol)
	}
	switch o.exposure {
	case "all", "localhost", "public", "interface":
	default:
		return fmt.Errorf("unknown exposure %q", o.exposure)
	}
	switch o.sortBy {
	case "port", "memory", "project", "user", "name", "protocol":
	default:
		return fmt.Errorf("unknown sort %q", o.sortBy)
	}
	if o.graceful {
		o.force = false
	}
	if o.pid < 0 {
		return fmt.Errorf("PID must be positive")
	}
	if o.pid > 0 {
		p, e := killport.GetProcessDetails(o.pid)
		if e != nil {
			return e
		}
		o.startID = p.StartID
		if o.startID == "" {
			return fmt.Errorf("cannot read PID identity")
		}
	}
	if o.project != "" {
		o.project, err = absoluteDir(o.project)
		if err != nil {
			return err
		}
	}
	if (o.watch || o.observe) && o.confirm {
		return fmt.Errorf("watch modes do not accept --interactive; use --observe to review targets first")
	}
	if o.watch && o.observe {
		return fmt.Errorf("choose --watch or --observe")
	}
	if o.ui && (o.json || o.quiet || o.watch || o.observe) {
		return fmt.Errorf("--ui cannot be combined with --json, --quiet, --watch, or --observe")
	}
	args = fs.Args()
	start := time.Now()
	if o.bench && !o.quiet && !o.json {
		defer func() { fmt.Printf("Completed in %v\n", time.Since(start).Round(time.Microsecond)) }()
	}
	if len(args) > 0 && (args[0] == "free" || args[0] == "suggest") {
		return runFree(args, o)
	}
	projectMode := len(args) == 1 && (args[0] == "." || args[0] == "auto" || args[0] == "cwd" || args[0] == "here")
	var ports []int
	if !projectMode {
		ports, err = parsePortArgsWithResolver(args, o.service.ResolvePreset)
		if err != nil {
			return err
		}
	}
	if len(args) > 0 && !projectMode && len(ports) == 0 {
		return fmt.Errorf("no valid ports specified")
	}
	// Destructive flags must never silently fall back to the dashboard or a list.
	if (o.watch || o.observe) && len(ports) == 0 && !projectMode {
		return fmt.Errorf("watch requires ports/presets or a project target (.)")
	}
	if projectMode && o.project == "" {
		o.project, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	automaticUI := len(args) == 0 && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) && !o.list && !o.json && !o.quiet && !o.inspect && !o.dry
	if o.ui || automaticUI {
		return runDashboard(ports, o)
	}

	if o.watch || o.observe {
		if o.dry || o.inspect || o.list {
			return fmt.Errorf("use --observe for monitoring; --watch cannot be combined with read-only modes")
		}
		return runWatch(ports, o)
	}
	var ps []killport.Process
	if projectMode {
		all, e := discoverUnscoped(o)
		if e != nil {
			return e
		}
		owned, suggestions := detect.MatchProject(o.project, all)
		ps = filterProcesses(owned, o)
		// Guessed ports are reported but never automatically targeted.
		if len(suggestions) > 0 && !o.json && !o.quiet {
			fmt.Fprintln(os.Stderr, "Unverified port matches (skipped; use an explicit port to target):")
			for _, p := range suggestions {
				fmt.Fprintf(os.Stderr, "  %d/%s PID %d %s: %s\n", p.Port, p.Protocol, p.PID, p.Name, p.MatchReason)
			}
		}
	} else {
		ps, err = discover(ports, o)
		if err != nil {
			return err
		}
	}
	readonly := o.list || o.inspect || o.dry || len(args) == 0
	if readonly {
		return outputProcesses(ps, o)
	}
	if o.confirm {
		ps, err = confirmTargets(ps)
		if err != nil {
			return err
		}
	}
	rs := o.service.ExecuteTargets(ps, killOptions(o), o.allowProtected)
	if o.wait {
		seen := map[int]bool{}
		for _, r := range rs {
			seen[r.Port] = true
		}
		for _, port := range ports {
			if !seen[port] {
				if e := killport.WaitForFreeContext(o.ctx, port, o.waitTimeout, killport.Protocol(o.protocol)); e != nil {
					rs = append(rs, killport.KillResult{Port: port, Process: killport.Process{Port: port, Protocol: killport.Protocol(o.protocol)}, Error: e})
				}
			}
		}
	}
	if err = outputResults(rs, o); err != nil {
		return err
	}
	if resultsError(rs) {
		return fmt.Errorf("one or more targets could not be stopped or ports remain occupied")
	}
	return nil
}

func killOptions(o options) killport.KillOptions {
	return killport.KillOptions{Context: o.ctx, Protocol: killport.Protocol(o.protocol), Force: o.force, Timeout: o.timeout, KillTree: o.tree, WaitForFree: o.wait, WaitTimeout: o.waitTimeout, DryRun: o.dry || o.inspect || o.list}
}
func absoluteDir(dir string) (string, error) { return detect.AbsoluteDir(dir) }
func discoverUnscoped(o options) ([]killport.Process, error) {
	return o.service.Discover(o.ctx, killport.FindOptions{Protocol: killport.Protocol(o.protocol), IncludeTree: o.tree || o.inspect || o.verbose})
}
func discover(ports []int, o options) ([]killport.Process, error) {
	ps, e := o.service.DiscoverPorts(o.ctx, ports, killport.FindOptions{Protocol: killport.Protocol(o.protocol), IncludeTree: o.tree || o.inspect || o.verbose})
	if e != nil {
		return nil, e
	}
	return filterProcesses(restrictPorts(ps, ports), o), nil
}

func restrictPorts(ps []killport.Process, ports []int) []killport.Process {
	if len(ports) == 0 {
		return ps
	}
	set := map[int]bool{}
	for _, p := range ports {
		set[p] = true
	}
	var result []killport.Process
	for _, p := range ps {
		if set[p.Port] {
			result = append(result, p)
		}
	}
	return result
}
func filterProcesses(ps []killport.Process, o options) []killport.Process {
	var result []killport.Process
	for _, p := range ps {
		if o.pid > 0 && (p.PID != o.pid || p.StartID != o.startID) {
			continue
		}
		if o.name != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.CommandLine+" "+p.ContainerName), strings.ToLower(o.name)) {
			continue
		}
		if o.user != "" && p.User != o.user {
			continue
		}
		if o.project != "" && len(detect.FindProcessesByCWD(o.project, []killport.Process{p})) == 0 {
			continue
		}
		if o.project != "" {
			p.MatchReason = "working directory belongs to " + o.project
		}
		if o.exposure != "all" && o.exposure != "" && tui.Exposure(p.Address) != o.exposure {
			continue
		}
		result = append(result, p)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		switch o.sortBy {
		case "memory":
			if a.MemoryRSS != b.MemoryRSS {
				return a.MemoryRSS > b.MemoryRSS
			}
		case "project":
			if a.Cwd != b.Cwd {
				return a.Cwd < b.Cwd
			}
		case "user":
			if a.User != b.User {
				return a.User < b.User
			}
		case "name":
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		case "protocol":
			if a.Protocol != b.Protocol {
				return a.Protocol < b.Protocol
			}
		}
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		return a.PID < b.PID
	})
	return result
}
func outputProcesses(ps []killport.Process, o options) error {
	if o.quiet {
		return nil
	}
	if ps == nil {
		ps = []killport.Process{}
	}
	if o.json {
		return json.NewEncoder(os.Stdout).Encode(ps)
	}
	if len(ps) == 0 {
		fmt.Println("No matching listening processes.")
		return nil
	}
	if o.dry {
		fmt.Println("[DRY RUN] Preview only; no processes will be stopped.")
	}
	fmt.Printf("%-7s %-5s %-8s %-24s %-10s %-12s %s\n", "PORT", "PROTO", "PID", "NAME / CONTAINER", "MEMORY", "USER", "ADDRESS")
	for _, p := range ps {
		name := p.Name
		if p.ContainerName != "" {
			name = "docker:" + p.ContainerName
		}
		fmt.Printf("%-7d %-5s %-8d %-24s %-10s %-12s %s\n", p.Port, p.Protocol, p.PID, tui.Clean(name), p.MemoryHuman, tui.Clean(p.User), tui.Clean(p.Address))
		if o.verbose || o.inspect {
			fmt.Printf("  Command: %s\n  CWD: %s\n  Parent: %d | Children: %v | Identity: %s\n", tui.Clean(p.CommandLine), tui.Clean(p.Cwd), p.PPID, p.ChildrenPIDs, p.StartID)
		}
		if p.MatchReason != "" {
			fmt.Println("  Match:", p.MatchReason)
		}
	}
	return nil
}
func outputResults(rs []killport.KillResult, o options) error {
	if o.quiet {
		return nil
	}
	// Errors need a string representation; encoding an error interface loses its message.
	type result struct {
		PID            int              `json:"pid"`
		Name           string           `json:"name"`
		Port           int              `json:"port"`
		Process        killport.Process `json:"process"`
		Killed         bool             `json:"killed"`
		Signal         string           `json:"signal,omitempty"`
		ChildrenKilled int              `json:"children_killed,omitempty"`
		Error          string           `json:"error,omitempty"`
	}
	out := []result{}
	for _, r := range rs {
		message := ""
		if r.Error != nil {
			message = r.Error.Error()
		}
		out = append(out, result{PID: r.Process.PID, Name: r.Process.Name, Port: r.Port, Process: r.Process, Killed: r.Killed, Signal: r.Signal, ChildrenKilled: r.ChildrenKilled, Error: message})
		if !o.json {
			if r.Error != nil {
				fmt.Printf("Failed PID %d port %d: %v\n", r.Process.PID, r.Port, r.Error)
			} else if r.Signal == "DRY-RUN" {
				fmt.Printf("Would stop PID %d on port %d\n", r.Process.PID, r.Port)
			} else {
				fmt.Printf("Stopped %s (PID %d) on %d/%s [%s]\n", tui.Clean(r.Process.Name), r.Process.PID, r.Port, r.Process.Protocol, r.Signal)
			}
		}
		if o.notify && r.Killed {
			notify.SendNotification("kport", fmt.Sprintf("Stopped %s on port %d", r.Process.Name, r.Port))
		}
	}
	if o.json {
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	return nil
}
func confirmTargets(ps []killport.Process) ([]killport.Process, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("--interactive requires a terminal")
	}
	scanner := bufio.NewScanner(os.Stdin)
	var approved []killport.Process
	seen := map[string]bool{}
	answers := map[string]bool{}
	for _, p := range ps {
		k := identity(p)
		if !seen[k] {
			fmt.Fprintf(os.Stderr, "Stop %s PID %d (including all its ports)? [y/N]: ", tui.Clean(p.Name), p.PID)
			if !scanner.Scan() {
				return nil, fmt.Errorf("confirmation input closed")
			}
			answers[k] = strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") || strings.EqualFold(strings.TrimSpace(scanner.Text()), "yes")
			seen[k] = true
		}
		if answers[k] {
			approved = append(approved, p)
		}
	}
	return approved, nil
}
func runFree(args []string, o options) error {
	start, end, count := 3000, 3050, 5
	if len(args) > 2 {
		return fmt.Errorf("usage: kport free [port|range]")
	}
	if len(args) == 2 {
		parts := strings.Split(strings.TrimPrefix(args[1], ":"), "-")
		var e error
		start, e = strconv.Atoi(parts[0])
		if e != nil {
			return fmt.Errorf("invalid port")
		}
		end = start + 50
		if end > 65535 {
			end = 65535
		}
		if len(parts) == 2 {
			end, e = strconv.Atoi(parts[1])
			if e != nil {
				return e
			}
			count = end - start + 1
		}
		if len(parts) > 2 || start < 1 || start > 65535 || end < start || end > 65535 {
			return fmt.Errorf("invalid port range")
		}
	}
	ps, e := killport.FindFreePorts(start, end, count, killport.Protocol(o.protocol))
	if e != nil {
		return e
	}
	if o.quiet {
		return nil
	}
	if o.json {
		return json.NewEncoder(os.Stdout).Encode(ps)
	}
	for _, p := range ps {
		fmt.Println(p)
	}
	return nil
}
func printCustomHelp() {
	fmt.Fprintln(os.Stderr, `kport - port manager
Usage: kport [options] [ports | presets | .]
  kport                          Open the live dashboard
  kport --inspect 3000           Inspect without stopping anything
  kport -W 3000                  Stop and verify port release
  kport .                        Stop verified processes in this project
  kport --observe 3000           Monitor occupancy without killing
  kport -w 3000 --project .      Auto-stop only this project's processes
  kport free 3000                Suggest free ports (one per line)
  kport completion <shell>       Generate shell completions
Dashboard: / search, Enter details, Space select, x stop, F force, ? help`)
}
