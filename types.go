package killport

import (
	"github.com/tyzrex/killport/types"
)

// Type aliases for seamless root package usage
type (
	Protocol    = types.Protocol
	Process     = types.Process
	KillResult  = types.KillResult
	FindOptions = types.FindOptions
	KillOptions = types.KillOptions
)

const (
	ProtocolTCP = types.ProtocolTCP
	ProtocolUDP = types.ProtocolUDP
	ProtocolAll = types.ProtocolAll
)

// DefaultKillOptions returns standard options with graceful fallback and ProtocolAll.
func DefaultKillOptions() KillOptions {
	return types.DefaultKillOptions()
}

// DefaultFindOptions returns standard options matching all protocols.
func DefaultFindOptions() FindOptions {
	return types.DefaultFindOptions()
}
