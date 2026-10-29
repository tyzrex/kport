package types

import (
	"context"
	"time"
)

// Protocol represents a network protocol filter.
type Protocol string

const (
	// ProtocolTCP targets TCP sockets.
	ProtocolTCP Protocol = "tcp"
	// ProtocolUDP targets UDP sockets.
	ProtocolUDP Protocol = "udp"
	// ProtocolAll targets both TCP and UDP sockets.
	ProtocolAll Protocol = "all"
)

// Normalize returns the canonical lowercase protocol or ProtocolAll by default.
func (p Protocol) Normalize() Protocol {
	switch p {
	case "tcp", "TCP":
		return ProtocolTCP
	case "udp", "UDP":
		return ProtocolUDP
	case "all", "ALL", "both", "BOTH", "":
		return ProtocolAll
	default:
		return ProtocolAll
	}
}

// Matches returns true if this protocol matches the target protocol.
func (p Protocol) Matches(target Protocol) bool {
	norm := p.Normalize()
	if norm == ProtocolAll {
		return true
	}
	return norm == target.Normalize()
}

// Process represents an active process bound to a network port with extended details.
type Process struct {
	StartID       string   `json:"start_id,omitempty"`
	ContainerID   string   `json:"container_id,omitempty"`
	ContainerName string   `json:"container_name,omitempty"`
	MatchReason   string   `json:"match_reason,omitempty"`
	PID           int      `json:"pid"`
	PPID          int      `json:"ppid,omitempty"`
	Name          string   `json:"name"`
	CommandLine   string   `json:"command_line,omitempty"`
	Exe           string   `json:"exe,omitempty"`
	Cwd           string   `json:"cwd,omitempty"`
	User          string   `json:"user,omitempty"`
	Port          int      `json:"port"`
	Protocol      Protocol `json:"protocol"`
	Address       string   `json:"address"`
	State         string   `json:"state,omitempty"`
	MemoryRSS     uint64   `json:"memory_rss_bytes,omitempty"`
	MemoryHuman   string   `json:"memory_human,omitempty"`
	ChildrenPIDs  []int    `json:"children_pids,omitempty"`
}

// KillResult represents the result of attempting to kill a process on a port.
type KillResult struct {
	Port           int     `json:"port"`
	Process        Process `json:"process"`
	Killed         bool    `json:"killed"`
	Signal         string  `json:"signal,omitempty"`
	ChildrenKilled int     `json:"children_killed,omitempty"`
	Error          error   `json:"error,omitempty"`
}

// FindOptions configures the process discovery behavior.
type FindOptions struct {
	// Context cancels discovery and bounds external commands. Nil uses Background.
	Context context.Context `json:"-"`
	// Protocol restricts discovery to TCP, UDP, or All (default: ProtocolAll).
	Protocol Protocol `json:"protocol"`

	// NameFilter restricts discovery to processes matching this name (case-insensitive substring).
	NameFilter string `json:"name_filter,omitempty"`

	// IncludeTree retrieves child PIDs and process tree information.
	IncludeTree bool `json:"include_tree"`
}

// KillOptions configures the process killing behavior.
type KillOptions struct {
	// Context cancels pending actions and graceful waits. Nil uses Background.
	Context context.Context `json:"-"`
	// TreeTargets pins an approved descendant snapshot. Nil discovers descendants.
	TreeTargets map[int]string `json:"-"`
	// ExpectedStartID guards internal signal escalation against PID reuse.
	ExpectedStartID string `json:"-"`
	// Protocol restricts killing to TCP, UDP, or All (default: ProtocolAll).
	Protocol Protocol `json:"protocol"`

	// Force sends SIGKILL (or immediate terminate on Windows) without attempting graceful termination.
	Force bool `json:"force"`

	// Timeout specifies how long to wait for a process to terminate gracefully before sending SIGKILL.
	// Default is 1500ms if Force is false.
	Timeout time.Duration `json:"timeout"`

	// KillTree kills the process and all of its descendant child processes.
	KillTree bool `json:"kill_tree"`

	// WaitForFree polls until the port is completely released by the OS.
	WaitForFree bool `json:"wait_for_free"`

	// WaitTimeout is the maximum duration to wait for port release when WaitForFree is true (default: 5s).
	WaitTimeout time.Duration `json:"wait_timeout"`

	// DryRun finds the processes without sending any kill signals.
	DryRun bool `json:"dry_run"`
}

// DefaultKillOptions returns standard options with graceful fallback and ProtocolAll.
func DefaultKillOptions() KillOptions {
	return KillOptions{
		Protocol:    ProtocolAll,
		Force:       false,
		Timeout:     1500 * time.Millisecond,
		KillTree:    false,
		WaitForFree: false,
		WaitTimeout: 5 * time.Second,
		DryRun:      false,
	}
}

// DefaultFindOptions returns standard options matching all protocols.
func DefaultFindOptions() FindOptions {
	return FindOptions{
		Protocol: ProtocolAll,
	}
}
