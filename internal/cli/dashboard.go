package cli

import (
	"fmt"
	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/notify"
	"github.com/tyzrex/kport/internal/tui"
)

func runDashboard(ports []int, o options) error {
	opts := killOptions(o)
	return tui.Run(tui.Options{
		Interval: o.interval, Sort: o.sortBy, Exposure: o.exposure, DryRun: o.dry || o.inspect || o.list, Force: o.force, Tree: o.tree, Wait: o.wait, NoColor: o.noColor,
		Discover: func() ([]killport.Process, error) {
			all := o
			all.tree = true
			all.exposure = "all"
			everything, e := discoverUnscoped(all)
			if e != nil {
				return nil, e
			}
			ps := filterProcesses(restrictPorts(everything, ports), all)

			return expandTargets(ps, everything), nil
		},
		Execute: func(ps []killport.Process, force bool) []killport.KillResult {
			opts.Force = force
			rs := executeTargets(ps, opts, o.allowProtected)
			if o.notify {
				for _, r := range rs {
					if r.Killed {
						notify.SendNotification("kport", fmt.Sprintf("Stopped %s on port %d", r.Process.Name, r.Port))
					}
				}
			}
			return rs
		},
		Inspect: func(pid int) (killport.Process, error) {
			if pid <= 0 {
				return killport.Process{}, fmt.Errorf("no parent")
			}
			return killport.GetProcessDetails(pid)
		},
	})
}
