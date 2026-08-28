package killport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tyzrex/killport/internal/killer"
	"github.com/tyzrex/killport/internal/proc"
)

// Presets maps common framework / service names to their default development ports.
var Presets = map[string][]int{
	"web":        {3000, 3001, 5173, 8000, 8080, 4200, 5000, 8888, 1234, 4321, 9000},
	"dev":        {3000, 3001, 5173, 8000, 8080, 4200, 5000, 8888, 1234, 4321, 9000},
	"db":         {5432, 3306, 27017, 6379, 1433, 9200, 9300},
	"database":   {5432, 3306, 27017, 6379, 1433, 9200, 9300},
	"next":       {3000},
	"nextjs":     {3000},
	"react":      {3000},
	"remix":      {3000},
	"vite":       {5173},
	"astro":      {4321},
	"svelte":     {5173},
	"vue":        {8080, 5173},
	"angular":    {4200},
	"django":     {8000},
	"fastapi":    {8000},
	"flask":      {5000},
	"rails":      {3000},
	"redis":      {6379},
	"postgres":   {5432},
	"postgresql": {5432},
	"mysql":      {3306},
	"mariadb":    {3306},
	"mongo":      {27017},
	"mongodb":    {27017},
	"elastic":    {9200, 9300},
}

// ProtectedPorts is a list of critical network ports with their description.
var ProtectedPorts = map[int]string{
	22:  "SSH (Secure Shell)",
	53:  "DNS (Domain Name System)",
	67:  "DHCP Server",
	68:  "DHCP Client",
	123: "NTP (Network Time Protocol)",
}

// ResolvePreset returns the ports for a known framework/service preset name.
func ResolvePreset(name string) ([]int, bool) {
	ports, ok := Presets[strings.ToLower(strings.TrimSpace(name))]
	return ports, ok
}

// IsProtectedPort checks if a port belongs to critical system services.
func IsProtectedPort(port int) (string, bool) {
	desc, ok := ProtectedPorts[port]
	return desc, ok
}

// Kill kills processes listening on one or more specified ports using default options.
// Example:
//
//	results, err := killport.Kill(3000)
//	results, err := killport.Kill(3000, 8080, 5432)
func Kill(ports ...int) ([]KillResult, error) {
	return KillPorts(ports, DefaultKillOptions())
}

// KillPort kills processes listening on a single port with custom options.
func KillPort(port int, opts KillOptions) ([]KillResult, error) {
	return KillPorts([]int{port}, opts)
}

// KillPorts kills processes listening on the given list of ports with custom options.
func KillPorts(ports []int, opts KillOptions) ([]KillResult, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports specified to kill")
	}

	processes, err := proc.FindProcesses(ports, FindOptions{Protocol: opts.Protocol})
	if err != nil {
		return nil, fmt.Errorf("failed to discover processes on ports %v: %w", ports, err)
	}

	if len(processes) == 0 {
		return nil, nil
	}

	var results []KillResult
	killedPIDs := make(map[int]bool)

	for _, p := range processes {
		if killedPIDs[p.PID] {
			results = append(results, KillResult{
				Port:    p.Port,
				Process: p,
				Killed:  true,
				Signal:  "ALREADY_KILLED",
				Error:   nil,
			})
			continue
		}

		res := killer.KillProcess(p, opts)
		if res.Killed {
			killedPIDs[p.PID] = true
		}
		results = append(results, res)
	}

	// If WaitForFree is enabled, poll until ports are completely free
	if opts.WaitForFree {
		waitTimeout := opts.WaitTimeout
		if waitTimeout <= 0 {
			waitTimeout = 5 * time.Second
		}
		for _, port := range ports {
			_ = WaitForFree(port, waitTimeout, opts.Protocol)
		}
	}

	return results, nil
}

// Find returns all active processes listening on the given ports using default options.
func Find(ports ...int) ([]Process, error) {
	return FindPorts(ports, DefaultFindOptions())
}

// FindPort returns processes listening on a single port with custom options.
func FindPort(port int, opts FindOptions) ([]Process, error) {
	return FindPorts([]int{port}, opts)
}

// FindPorts returns processes listening on multiple ports with custom options.
func FindPorts(ports []int, opts FindOptions) ([]Process, error) {
	return proc.FindProcesses(ports, opts)
}

// FindAll returns all listening processes across all ports on the system.
func FindAll(opts ...FindOptions) ([]Process, error) {
	opt := DefaultFindOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	return proc.FindAllProcesses(opt)
}

// FindByName searches for listening processes whose name or command line matches the given query.
func FindByName(name string, opts ...FindOptions) ([]Process, error) {
	opt := DefaultFindOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt.NameFilter = name
	return proc.FindAllProcesses(opt)
}

// IsPortInUse returns true if at least one process is listening on the specified port.
func IsPortInUse(port int, protocol ...Protocol) (bool, error) {
	proto := ProtocolAll
	if len(protocol) > 0 {
		proto = protocol[0]
	}
	procs, err := FindPort(port, FindOptions{Protocol: proto})
	if err != nil {
		return false, err
	}
	return len(procs) > 0, nil
}

// WaitForFree blocks until the specified port is no longer in use or until timeout occurs.
func WaitForFree(port int, timeout time.Duration, protocol ...Protocol) error {
	proto := ProtocolAll
	if len(protocol) > 0 {
		proto = protocol[0]
	}

	deadline := time.Now().Add(timeout)
	interval := 30 * time.Millisecond

	for time.Now().Before(deadline) {
		inUse, err := IsPortInUse(port, proto)
		if err == nil && !inUse {
			return nil
		}
		time.Sleep(interval)
		if interval < 150*time.Millisecond {
			interval += 15 * time.Millisecond
		}
	}

	inUse, err := IsPortInUse(port, proto)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("timeout waiting for port %d to be released after %v", port, timeout)
	}
	return nil
}

// KillProcess kills a specific Process instance directly.
func KillProcess(p Process, opts ...KillOptions) KillResult {
	opt := DefaultKillOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	return killer.KillProcess(p, opt)
}

// Watch continuously monitors the specified ports and kills any process that binds to them.
func Watch(ctx context.Context, ports []int, opts KillOptions, interval time.Duration, onKill func(result KillResult)) error {
	if len(ports) == 0 {
		return fmt.Errorf("no ports provided to watch")
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			results, err := KillPorts(ports, opts)
			if err == nil && len(results) > 0 && onKill != nil {
				for _, r := range results {
					if r.Killed {
						onKill(r)
					}
				}
			}
		}
	}
}

// GetProcessDetails retrieves detailed metadata about a given PID.
func GetProcessDetails(pid int) (Process, error) {
	name, cmdline, exe, user, ppid, cwd, rss := proc.GetProcessDetails(pid)
	if name == "" && exe == "" {
		return Process{}, fmt.Errorf("process %d not found or not readable", pid)
	}
	children := proc.FindProcessChildren(pid)
	return Process{
		PID:          pid,
		PPID:         ppid,
		Name:         name,
		CommandLine:  cmdline,
		Exe:          exe,
		Cwd:          cwd,
		User:         user,
		MemoryRSS:    rss,
		MemoryHuman:  proc.FormatBytes(rss),
		ChildrenPIDs: children,
	}, nil
}
