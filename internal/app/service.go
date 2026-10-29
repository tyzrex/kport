package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/config"
	"github.com/tyzrex/kport/internal/docker"
	"github.com/tyzrex/kport/internal/proc"
)

// ExpandTargets includes every socket affected by stopping the selected owner.
func ExpandTargets(targets, all []killport.Process) []killport.Process {
	selected := map[string]killport.Process{}
	for _, p := range targets {
		selected[Identity(p)] = p
	}
	var expanded []killport.Process
	found := map[string]bool{}
	for _, p := range all {
		if original, ok := selected[Identity(p)]; ok {
			p.MatchReason = original.MatchReason
			expanded = append(expanded, p)
			found[Identity(p)] = true
		}
	}
	for k, p := range selected {
		if !found[k] {
			expanded = append(expanded, p)
		}
	}
	return expanded
}

// ExecuteTargets is shared by CLI, dashboard, project mode, and watch mode.
func (s *Service) ExecuteTargets(ps []killport.Process, opts killport.KillOptions, allowProtected bool) []killport.KillResult {
	if len(ps) == 0 {
		return nil
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	all, discoveryErr := s.Discover(opts.Context, killport.FindOptions{IncludeTree: opts.KillTree})
	ps = ExpandTargets(ps, all)
	// Pin descendants before evaluating protection. Execution must not rediscover
	// additional targets after the protection decision.
	treeIDs := map[int]string{}
	if opts.KillTree && !opts.DryRun {
		for _, p := range ps {
			for _, pid := range p.ChildrenPIDs {
				treeIDs[pid] = proc.StartIdentity(pid)
			}
		}
	}

	var results []killport.KillResult
	seen := map[string]killport.KillResult{}
	for _, p := range ps {
		key := Identity(p)
		if r, ok := seen[key]; ok {
			r.Port = p.Port
			r.Process = p
			results = append(results, r)
			continue
		}
		r := killport.KillResult{Port: p.Port, Process: p}
		if !opts.DryRun {
			if discoveryErr != nil {
				r.Error = discoveryErr
				results = append(results, r)
				continue
			}
			if p.ContainerID == "" && p.StartID == "" {
				r.Error = fmt.Errorf("cannot verify PID %d identity; refresh with sufficient permissions", p.PID)
				results = append(results, r)
				continue
			}
			if e := s.CheckProtection(p, all, opts.KillTree, allowProtected); e != nil {
				r.Error = e
				seen[key] = r
				results = append(results, r)
				continue
			}
		}
		if p.ContainerID != "" {
			r.Signal = "DOCKER_STOP"
			if opts.Force {
				r.Signal = "DOCKER_KILL"
			}
			if opts.DryRun {
				r.Signal = "DRY-RUN"
			} else {
				// Confirm that this exact container still owns the selected host mapping.
				owns, e := docker.OwnsContext(opts.Context, p.ContainerID, p.Port, p.Protocol, p.StartID)
				if e != nil {
					r.Error = e
				} else if !owns {
					r.Error = fmt.Errorf("container mapping changed; refresh before stopping")
				} else {
					r.Error = docker.StopContext(opts.Context, p.ContainerID, opts.Force, opts.Timeout)
					r.Killed = r.Error == nil
				}
			}
		} else if !opts.DryRun && docker.IsBackend(p) {
			r.Error = fmt.Errorf("cannot resolve container for shared Docker backend PID %d; use docker stop <container>", p.PID)
		} else {
			signalOpts := opts
			if opts.KillTree && !opts.DryRun {
				signalOpts.TreeTargets = map[int]string{}
				for _, pid := range p.ChildrenPIDs {
					signalOpts.TreeTargets[pid] = treeIDs[pid]
					if signalOpts.TreeTargets[pid] == "" {
						r.Error = fmt.Errorf("cannot verify child PID %d", pid)
						break
					}
				}
				if r.Error != nil {
					seen[key] = r
					results = append(results, r)
					continue
				}
			}
			signalOpts.WaitForFree = false
			r = killport.KillProcess(p, signalOpts)
		}
		seen[key] = r
		results = append(results, r)
	}
	if opts.WaitForFree && !opts.DryRun {
		timeout := opts.WaitTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		for i := range results {
			if results[i].Error == nil {
				if e := killport.WaitForFreeContext(opts.Context, results[i].Port, timeout, results[i].Process.Protocol); e != nil {
					results[i].Error = e
				}
			}
		}
	}
	return results
}
func (s *Service) CheckProtection(p killport.Process, all []killport.Process, tree, allow bool) error {
	if p.ContainerID == "" && (p.PID <= 1 || p.PID == os.Getpid()) {
		return fmt.Errorf("protected PID %d", p.PID)
	}
	if allow {
		return nil
	}
	children := map[int]bool{}
	if tree {
		for _, c := range p.ChildrenPIDs {
			children[c] = true
		}
	}
	for _, other := range append(all, p) {
		same := other.PID == p.PID
		if p.ContainerID != "" {
			same = other.ContainerID == p.ContainerID
		}
		if same || children[other.PID] {
			if desc, ok := s.protectedPort(other.Port); ok {
				return fmt.Errorf("port %d protected (%s); use --force-protected", other.Port, desc)
			}
		}
	}
	return nil
}
func ResultsError(rs []killport.KillResult) bool {
	for _, r := range rs {
		if r.Error != nil {
			return true
		}
	}
	return false
}
func Identity(p killport.Process) string {
	if p.ContainerID != "" {
		return "container:" + p.ContainerID + ":" + p.StartID
	}
	return fmt.Sprintf("pid:%d:%s", p.PID, p.StartID)
}

// Service owns configuration for one application session without global mutation.
type Service struct {
	protected map[int]string
	presets   map[string][]int
}

func New(cfg config.Config) *Service {
	s := &Service{protected: map[int]string{}, presets: map[string][]int{}}
	for p, label := range killport.ProtectedPorts {
		s.protected[p] = label
	}
	for p, ports := range killport.Presets {
		s.presets[strings.ToLower(strings.TrimSpace(p))] = append([]int(nil), ports...)
	}
	for _, p := range cfg.ProtectedPorts {
		s.protected[p] = "configured protection"
	}
	for p, ports := range cfg.Presets {
		s.presets[strings.ToLower(strings.TrimSpace(p))] = append([]int(nil), ports...)
	}
	return s
}
func (s *Service) protectedPort(port int) (string, bool) {
	if s == nil {
		return killport.IsProtectedPort(port)
	}
	v, ok := s.protected[port]
	return v, ok
}
func (s *Service) ResolvePreset(name string) ([]int, bool) {
	if s == nil {
		return killport.ResolvePreset(name)
	}
	v, ok := s.presets[strings.ToLower(strings.TrimSpace(name))]
	return append([]int(nil), v...), ok
}
func (s *Service) Discover(ctx context.Context, opts killport.FindOptions) ([]killport.Process, error) {
	return s.DiscoverPorts(ctx, nil, opts)
}

// DiscoverPorts includes optional Docker mappings; an empty selection discovers all ports.
func (s *Service) DiscoverPorts(ctx context.Context, ports []int, opts killport.FindOptions) ([]killport.Process, error) {
	opts.Context = ctx
	var ps []killport.Process
	var err error
	if len(ports) == 0 {
		ps, err = killport.FindAll(opts)
	} else {
		ps, err = killport.FindPorts(ports, opts)
	}
	if err != nil {
		return nil, err
	}
	ps, err = docker.EnrichContext(ctx, ps)
	if err != nil {
		return nil, err
	}
	return restrictDiscovery(ps, ports, opts.Protocol), nil
}

// Docker publishes mappings independently of OS discovery, so apply the query
// restrictions again after enrichment.
func restrictDiscovery(ps []killport.Process, ports []int, protocol killport.Protocol) []killport.Process {
	var result []killport.Process
	for _, p := range ps {
		if protocol.Matches(p.Protocol) && proc.MatchPort(ports, p.Port) {
			result = append(result, p)
		}
	}
	return result
}
