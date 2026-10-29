// Package docker resolves published host ports to containers without signaling
// Docker's shared networking backend.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tyzrex/kport/internal/proc"
	"github.com/tyzrex/kport/types"
)

type ContainerInfo struct{ ID, Name, Ports, Cwd, StartedAt string }
type Binding struct {
	Container ContainerInfo
	HostIP    string
	Port      int
	Protocol  types.Protocol
}

func Bindings() ([]Binding, error) { return BindingsContext(context.Background()) }
func BindingsContext(ctx context.Context) ([]Binding, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, e := proc.Output(ctx, "docker", "ps", "-q", "--no-trunc")
	if e != nil {
		return nil, e
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return nil, nil
	}
	out, e = proc.Output(ctx, "docker", append([]string{"inspect"}, ids...)...)
	if e != nil {
		return nil, e
	}
	return parseBindings(out)
}
func parseBindings(out []byte) ([]Binding, error) {
	var objects []struct {
		ID    string `json:"Id"`
		Name  string
		State struct {
			StartedAt string
			Running   *bool
		}
		Config          struct{ Labels map[string]string }
		NetworkSettings struct {
			Ports map[string][]struct{ HostIP, HostPort string }
		}
	}
	if e := json.Unmarshal(out, &objects); e != nil {
		return nil, e
	}
	var result []Binding
	for _, c := range objects {
		if c.State.Running != nil && !*c.State.Running {
			continue
		}
		for key, bindings := range c.NetworkSettings.Ports {
			parts := strings.Split(key, "/")
			if len(parts) != 2 || (parts[1] != "tcp" && parts[1] != "udp") {
				continue
			}
			for _, b := range bindings {
				port, e := strconv.Atoi(b.HostPort)
				if e != nil || port < 1 || port > 65535 {
					continue
				}
				result = append(result, Binding{Container: ContainerInfo{ID: c.ID, Name: strings.TrimPrefix(c.Name, "/"), Ports: b.HostPort, Cwd: c.Config.Labels["com.docker.compose.project.working_dir"], StartedAt: c.State.StartedAt}, HostIP: b.HostIP, Port: port, Protocol: types.Protocol(parts[1])})
			}
		}
	}
	return result, nil
}
func IsBackend(p types.Process) bool {
	text := strings.ToLower(p.Name + " " + p.Exe)
	for _, name := range []string{"docker-proxy", "com.docker.backend", "rootlesskit", "rootlessport", "vpnkit"} {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}
func endpointHost(address string) string {
	host, _, e := net.SplitHostPort(address)
	if e != nil {
		if i := strings.LastIndex(address, ":"); i >= 0 {
			host = address[:i]
		}
	}
	host = strings.Trim(host, "[]")
	if host == "*" {
		return "*"
	}
	return host
}
func Enrich(ps []types.Process) []types.Process {
	result, _ := EnrichContext(context.Background(), ps)
	return result
}
func EnrichContext(ctx context.Context, ps []types.Process) ([]types.Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	bindings, e := BindingsContext(ctx)
	if ctx.Err() != nil {
		return ps, ctx.Err()
	}
	// Docker is optional; an unavailable engine does not hide ordinary listeners.
	if e != nil {
		return ps, nil
	}
	return enrichBindings(ps, bindings), nil
}
func enrichBindings(ps []types.Process, bindings []Binding) []types.Process {
	seen := map[int]bool{}
	for i, p := range ps {
		if !IsBackend(p) {
			continue
		}
		var matches []int
		for j, b := range bindings {
			if b.Port == p.Port && b.Protocol == p.Protocol {
				host := endpointHost(p.Address)
				if host == b.HostIP || host == "*" {
					matches = append(matches, j)
				}
			}
		}
		if len(matches) == 0 {
			continue
		}
		// A wildcard backend may serve several containers; replace it with exact
		// published endpoints rather than assigning an arbitrary container.
		for _, j := range matches {
			seen[j] = true
		}
		b := bindings[matches[0]]
		ps[i].StartID = b.Container.StartedAt
		ps[i].ContainerID = b.Container.ID
		ps[i].ContainerName = b.Container.Name
		ps[i].Cwd = b.Container.Cwd
		ps[i].Port = b.Port
		ps[i].Protocol = b.Protocol
		ps[i].Address = net.JoinHostPort(b.HostIP, strconv.Itoa(b.Port))
		for _, j := range matches[1:] {
			b := bindings[j]
			copy := ps[i]
			copy.StartID = b.Container.StartedAt
			copy.ContainerID = b.Container.ID
			copy.ContainerName = b.Container.Name
			copy.Cwd = b.Container.Cwd
			copy.Address = net.JoinHostPort(b.HostIP, strconv.Itoa(b.Port))
			ps = append(ps, copy)
		}
	}
	for i, b := range bindings {
		if !seen[i] {
			ps = append(ps, types.Process{Name: "docker:" + b.Container.Name, StartID: b.Container.StartedAt, ContainerID: b.Container.ID, ContainerName: b.Container.Name, Cwd: b.Container.Cwd, Port: b.Port, Protocol: b.Protocol, Address: net.JoinHostPort(b.HostIP, strconv.Itoa(b.Port)), State: "PUBLISHED"})
		}
	}
	return ps
}
func Owns(id string, port int, protocol types.Protocol, startID string) (bool, error) {
	return OwnsContext(context.Background(), id, port, protocol, startID)
}
func OwnsContext(ctx context.Context, id string, port int, protocol types.Protocol, startID string) (bool, error) {
	bindings, e := BindingsContext(ctx)
	if e != nil {
		return false, e
	}
	for _, b := range bindings {
		if b.Container.ID == id && b.Port == port && b.Protocol == protocol && b.Container.StartedAt == startID {
			return true, nil
		}
	}
	return false, nil
}
func GetContainerByPort(port int) (ContainerInfo, bool) {
	bindings, e := Bindings()
	if e != nil {
		return ContainerInfo{}, false
	}
	var found ContainerInfo
	for _, b := range bindings {
		if b.Port == port {
			if found.ID != "" && found.ID != b.Container.ID {
				return ContainerInfo{}, false
			}
			found = b.Container
		}
	}
	return found, found.ID != ""
}
func StopContainer(id string) error { return Stop(id, false, 2*time.Second) }
func Stop(id string, force bool, timeout time.Duration) error {
	return StopContext(context.Background(), id, force, timeout)
}
func StopContext(ctx context.Context, id string, force bool, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	args := []string{"stop", "-t", strconv.Itoa(max(1, int(math.Ceil(timeout.Seconds())))), id}
	if force {
		args = []string{"kill", id}
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.WaitDelay = 100 * time.Millisecond
	out, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("docker %s: %w: %s", args[0], e, strings.TrimSpace(string(out)))
	}
	return nil
}
