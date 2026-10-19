package cli

import (
	"fmt"
	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/docker"
	"os"
	"time"
)

// ExpandTargets includes every socket affected by stopping the selected owner.
func expandTargets(targets, all []killport.Process) []killport.Process {
	selected := map[string]killport.Process{}
	for _, p := range targets {
		selected[identity(p)] = p
	}
	var expanded []killport.Process
	found := map[string]bool{}
	for _, p := range all {
		if original, ok := selected[identity(p)]; ok {
			p.MatchReason = original.MatchReason
			expanded = append(expanded, p)
			found[identity(p)] = true
		}
	}
	for k, p := range selected {
		if !found[k] {
			expanded = append(expanded, p)
		}
	}
	return expanded
}

// executeTargets is shared by CLI, dashboard, project mode, and watch mode.
func executeTargets(ps []killport.Process, opts killport.KillOptions, allowProtected bool) []killport.KillResult {
	if len(ps) == 0 {
		return nil
	}
	all, discoveryErr := killport.FindAll(killport.FindOptions{IncludeTree: opts.KillTree})
	all = docker.Enrich(all)
	ps = expandTargets(ps, all)
	var results []killport.KillResult
	seen := map[string]killport.KillResult{}
	for _, p := range ps {
		key := identity(p)
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
			if e := checkProtection(p, all, opts.KillTree, allowProtected); e != nil {
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
				owns, e := docker.Owns(p.ContainerID, p.Port, p.Protocol, p.StartID)
				if e != nil {
					r.Error = e
				} else if !owns {
					r.Error = fmt.Errorf("container mapping changed; refresh before stopping")
				} else {
					r.Error = docker.Stop(p.ContainerID, opts.Force, opts.Timeout)
					r.Killed = r.Error == nil
				}
			}
		} else if !opts.DryRun && docker.IsBackend(p) {
			r.Error = fmt.Errorf("cannot resolve container for shared Docker backend PID %d; use docker stop <container>", p.PID)
		} else {
			signalOpts := opts
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
				if e := killport.WaitForFree(results[i].Port, timeout, results[i].Process.Protocol); e != nil {
					results[i].Error = e
				}
			}
		}
	}
	return results
}
func checkProtection(p killport.Process, all []killport.Process, tree, allow bool) error {
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
			if desc, ok := killport.IsProtectedPort(other.Port); ok {
				return fmt.Errorf("port %d protected (%s); use --force-protected", other.Port, desc)
			}
		}
	}
	return nil
}
func resultsError(rs []killport.KillResult) bool {
	for _, r := range rs {
		if r.Error != nil {
			return true
		}
	}
	return false
}
func identity(p killport.Process) string {
	if p.ContainerID != "" {
		return "container:" + p.ContainerID + ":" + p.StartID
	}
	return fmt.Sprintf("pid:%d:%s", p.PID, p.StartID)
}
