package cli

import (
	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/app"
)

func expandTargets(ps, all []killport.Process) []killport.Process { return app.ExpandTargets(ps, all) }
func checkProtection(p killport.Process, all []killport.Process, tree, allow bool) error {
	return (*app.Service)(nil).CheckProtection(p, all, tree, allow)
}
func resultsError(rs []killport.KillResult) bool { return app.ResultsError(rs) }
func identity(p killport.Process) string         { return app.Identity(p) }
