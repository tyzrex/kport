package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tyzrex/kport"
	"github.com/tyzrex/kport/internal/notify"
)

type watchEvent struct {
	Event    string            `json:"event"`
	Port     int               `json:"port,omitempty"`
	Protocol killport.Protocol `json:"protocol,omitempty"`
	Process  *killport.Process `json:"process,omitempty"`
	Message  string            `json:"message,omitempty"`
	Time     time.Time         `json:"time"`
}

func runWatch(ports []int, o options) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	emit := func(ev watchEvent) {
		ev.Time = time.Now()
		if !o.quiet {
			if o.json {
				_ = json.NewEncoder(os.Stdout).Encode(ev)
			} else {
				fmt.Printf("[%s] %s\n", ev.Event, ev.Message)
			}
		}
		if o.notify {
			notify.SendNotification("kport", ev.Message)
		}
	}
	return watchLoop(ctx, ports, o, emit)
}
func watchLoop(ctx context.Context, ports []int, o options, emit func(watchEvent)) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	previous := map[string]killport.Process{}
	initial := true
	stopped := map[string]killport.Process{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		ps, e := discover(ports, o)
		if e != nil {
			return e
		}
		current := map[string]killport.Process{}
		for _, p := range ps {
			k := fmt.Sprintf("%d/%s/%s", p.Port, p.Protocol, identity(p))
			current[k] = p
			if _, ok := previous[k]; !ok {
				ev := watchEvent{Event: "occupied", Port: p.Port, Protocol: p.Protocol, Process: &p, Message: fmt.Sprintf("%d/%s occupied by %s PID %d (parent %d)", p.Port, p.Protocol, p.Name, p.PID, p.PPID)}
				restartKey := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
				if old, ok := stopped[restartKey]; ok && identity(old) != identity(p) {
					ev.Event = "respawned"
					ev.Message = fmt.Sprintf("Port %d rebound by %s PID %d; previous PID %d; parent/supervisor PID %d", p.Port, p.Name, p.PID, old.PID, p.PPID)
				}
				emit(ev)
			}
		}
		for k, p := range previous {
			if _, ok := current[k]; !ok {
				emit(watchEvent{Event: "released", Port: p.Port, Protocol: p.Protocol, Message: fmt.Sprintf("PID %d released %d/%s (other listeners may remain)", p.PID, p.Port, p.Protocol)})
			}
		}
		// Report when the last matching owner leaves an endpoint.
		occupied := map[string]bool{}
		for _, p := range current {
			occupied[fmt.Sprintf("%d/%s", p.Port, p.Protocol)] = true
		}
		released := map[string]bool{}
		for _, p := range previous {
			k := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
			if !occupied[k] && !released[k] {
				released[k] = true
				emit(watchEvent{Event: "free", Port: p.Port, Protocol: p.Protocol, Message: fmt.Sprintf("%d/%s has no matching listeners", p.Port, p.Protocol)})
			}
		}
		if initial {
			for _, port := range ports {
				found := false
				for _, p := range ps {
					if p.Port == port {
						found = true
					}
				}
				if !found {
					emit(watchEvent{Event: "free", Port: port, Message: fmt.Sprintf("Port %d has no matching listeners", port)})
				}
			}
			initial = false
		}
		if ctx.Err() != nil {
			return nil
		}
		if !o.observe {
			for _, r := range executeTargets(ps, killOptions(o), o.allowProtected) {
				ev := watchEvent{Event: "stopped", Port: r.Port, Protocol: r.Process.Protocol, Process: &r.Process, Message: fmt.Sprintf("Stopped %s PID %d on %d/%s", r.Process.Name, r.Process.PID, r.Port, r.Process.Protocol)}
				if r.Error != nil {
					ev.Event = "error"
					ev.Message = r.Error.Error()
				} else {
					stopped[fmt.Sprintf("%d/%s", r.Port, r.Process.Protocol)] = r.Process
				}
				emit(ev)
			}
		}
		previous = current
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
