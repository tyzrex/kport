package types

import "time"

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

// Process represents an active process bound to a network port.
type Process struct {
	PID         int      `json:"pid"`
	Name        string   `json:"name"`
	CommandLine string   `json:"command_line,omitempty"`
	Exe         string   `json:"exe,omitempty"`
	User        string   `json:"user,omitempty"`
	Port        int      `json:"port"`
	Protocol    Protocol `json:"protocol"`
	Address     string   `json:"address"`
	State       string   `json:"state,omitempty"`
}

// KillResult represents the result of attempting to kill a process on a port.
type KillResult struct {
	Port     int      `json:"port"`
	Process  Process  `json:"process"`
	Killed   bool     `json:"killed"`
	Signal   string   `json:"signal,omitempty"`
	Error    error    `json:"error,omitempty"`
}

// FindOptions configures the process discovery behavior.
type FindOptions struct {
	// Protocol restricts discovery to TCP, UDP, or All (default: ProtocolAll).
	Protocol Protocol `json:"protocol"`
}

// KillOptions configures the process killing behavior.
type KillOptions struct {
	// Protocol restricts killing to TCP, UDP, or All (default: ProtocolAll).
	Protocol Protocol `json:"protocol"`

	// Force sends SIGKILL (or immediate terminate on Windows) without attempting graceful termination.
	Force bool `json:"force"`

	// Timeout specifies how long to wait for a process to terminate gracefully before sending SIGKILL.
	// Default is 1500ms if Force is false.
	Timeout time.Duration `json:"timeout"`

	// DryRun finds the processes without sending any kill signals.
	DryRun bool `json:"dry_run"`
}

// DefaultKillOptions returns standard options with graceful fallback and ProtocolAll.
func DefaultKillOptions() KillOptions {
	return KillOptions{
		Protocol: ProtocolAll,
		Force:    false,
		Timeout:  1500 * time.Millisecond,
		DryRun:   false,
	}
}

// DefaultFindOptions returns standard options matching all protocols.
func DefaultFindOptions() FindOptions {
	return FindOptions{
		Protocol: ProtocolAll,
	}
}
