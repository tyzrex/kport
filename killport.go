package killport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tyzrex/kport/internal/killer"
	"github.com/tyzrex/kport/internal/proc"
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

	for _, p := range ports {
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid port %d", p)
		}
	}
	processes, err := proc.FindProcesses(ports, FindOptions{Context: opts.Context, Protocol: opts.Protocol})
	if err != nil {
		return nil, fmt.Errorf("failed to discover processes on ports %v: %w", ports, err)
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
	if opts.WaitForFree && !opts.DryRun {
		waitTimeout := opts.WaitTimeout
		if waitTimeout <= 0 {
			waitTimeout = 5 * time.Second
		}
		for _, port := range ports {
			if err := WaitForFreeContext(opts.Context, port, waitTimeout, opts.Protocol); err != nil {
				return results, err
			}
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
	return WaitForFreeContext(context.Background(), port, timeout, protocol...)
}

// WaitForFreeContext verifies release until cancellation or timeout.
func WaitForFreeContext(ctx context.Context, port int, timeout time.Duration, protocol ...Protocol) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if port < 1 || port > 65535 || timeout <= 0 {
		return fmt.Errorf("valid port and positive wait timeout required")
	}
	proto := ProtocolAll
	if len(protocol) > 0 {
		proto = protocol[0]
	}
	deadline := time.Now().Add(timeout)
	interval := 30 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timeout or cancellation waiting for port %d: %w", port, err)
		}
		ps, err := FindPort(port, FindOptions{Context: ctx, Protocol: proto})
		inUse := len(ps) > 0
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("timeout or cancellation waiting for port %d: %w", port, ctx.Err())
			}
			return err
		}
		available, err := canBind(port, proto)
		if err != nil {
			return err
		}
		if available && !inUse {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timeout waiting for port %d to be released after %v", port, timeout)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout or cancellation waiting for port %d: %w", port, ctx.Err())
		case <-time.After(min(interval, remaining)):
		}
		interval = min(interval+15*time.Millisecond, 150*time.Millisecond)
	}
}

// KillProcess kills a specific Process instance directly.
func KillProcess(p Process, opts ...KillOptions) KillResult {
	opt := DefaultKillOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	r := killer.KillProcess(p, opt)
	if r.Error == nil && opt.WaitForFree && !opt.DryRun {
		timeout := opt.WaitTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		r.Error = WaitForFreeContext(opt.Context, p.Port, timeout, p.Protocol)
	}
	return r
}

// Watch continuously monitors the specified ports and kills any process that binds to them.
func Watch(ctx context.Context, ports []int, opts KillOptions, interval time.Duration, onKill func(result KillResult)) error {
	if len(ports) == 0 {
		return fmt.Errorf("no ports provided to watch")
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	opts.Context = ctx
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			results, err := KillPorts(ports, opts)
			if err != nil {
				return err
			}
			if len(results) > 0 && onKill != nil {
				for _, r := range results {
					onKill(r)
				}
			}
		}
	}
}

// GetProcessDetails retrieves detailed metadata about a given PID.
func GetProcessDetails(pid int) (Process, error) {
	return GetProcessDetailsContext(context.Background(), pid)
}

// GetProcessDetailsContext reads fresh metadata with cancellable OS queries.
func GetProcessDetailsContext(ctx context.Context, pid int) (Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	startID := proc.StartIdentity(pid)
	name, cmdline, exe, user, ppid, cwd, rss := proc.GetProcessDetailsContext(ctx, pid)
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	if name == "" && exe == "" {
		return Process{}, fmt.Errorf("process %d not found or not readable", pid)
	}
	if startID == "" || proc.StartIdentity(pid) != startID {
		return Process{}, fmt.Errorf("process %d changed or exited; refresh details", pid)
	}
	children := proc.FindProcessChildrenContext(ctx, pid)
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	return Process{
		StartID:      startID,
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

// FindFreePort searches starting from startPort for the next available unused port.
func FindFreePort(startPort int, protocol ...Protocol) (int, error) {
	ports, err := FindFreePorts(startPort, 65535, 1, protocol...)
	if err != nil || len(ports) == 0 {
		return 0, fmt.Errorf("no free port found starting from %d", startPort)
	}
	return ports[0], nil
}

// FindFreePorts returns up to count available unused ports within the [startPort, endPort] range.
func FindFreePorts(startPort, endPort, count int, protocol ...Protocol) ([]int, error) {
	if startPort <= 0 {
		startPort = 1024
	}
	if endPort > 65535 {
		endPort = 65535
	}
	if count <= 0 {
		count = 1
	}

	proto := ProtocolAll
	if len(protocol) > 0 {
		proto = protocol[0]
	}

	occupied := map[int]bool{}
	listeners, err := FindAll(FindOptions{Protocol: proto})
	if err != nil {
		return nil, err
	}
	for _, p := range listeners {
		occupied[p.Port] = true
	}
	var freePorts []int
	for p := startPort; p <= endPort; p++ {
		if occupied[p] {
			continue
		}
		available, err := canBind(p, proto)
		if err != nil {
			return nil, err
		}
		if available {
			freePorts = append(freePorts, p)
			if len(freePorts) >= count {
				break
			}
		}
	}

	if len(freePorts) == 0 {
		return nil, fmt.Errorf("no free ports found in range %d-%d", startPort, endPort)
	}
	return freePorts, nil
}

// SuggestNextPort returns the closest available port to an occupied port.
func SuggestNextPort(occupiedPort int, protocol ...Protocol) (int, error) {
	return FindFreePort(occupiedPort+1, protocol...)
}

// canBind verifies actual socket availability even if process ownership is hidden.
// A successful probe is a suggestion, not a reservation for a later bind.
func canBind(port int, proto Protocol) (bool, error) {
	if port < 1 || port > 65535 {
		return false, fmt.Errorf("port must be 1-65535")
	}
	networks := []string{"tcp4", "tcp6", "udp4", "udp6"}
	switch proto.Normalize() {
	case ProtocolTCP:
		networks = networks[:2]
	case ProtocolUDP:
		networks = networks[2:]
	}
	for _, network := range networks {
		hosts := []string{"", "127.0.0.1"}
		if strings.HasSuffix(network, "6") {
			hosts = []string{"", "::1"}
		}
		for _, host := range hosts {
			var closer interface{ Close() error }
			var err error
			address := net.JoinHostPort(host, strconv.Itoa(port))
			if strings.HasPrefix(network, "tcp") {
				closer, err = net.Listen(network, address)
			} else {
				closer, err = net.ListenPacket(network, address)
			}
			if err != nil {
				if errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EPROTONOSUPPORT) || errors.Is(err, syscall.EADDRNOTAVAIL) {
					continue
				}
				if errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, syscall.EACCES) {
					return false, nil
				}
				return false, err
			}
			closer.Close()
		}
	}
	return true, nil
}
