package docker

import (
	"github.com/tyzrex/kport/types"
	"testing"
)

func TestBindingsResolveComposeAndDoNotMislabelNativeProcesses(t *testing.T) {
	data := []byte(`[{"Id":"one","Name":"/api","Config":{"Labels":{"com.docker.compose.project.working_dir":"/work/app"}},"NetworkSettings":{"Ports":{"80/tcp":[{"HostIP":"127.0.0.1","HostPort":"3000"}],"53/udp":[{"HostIP":"0.0.0.0","HostPort":"5353"}]}}},{"Id":"two","Name":"/other","NetworkSettings":{"Ports":{"80/tcp":[{"HostIP":"127.0.0.2","HostPort":"3000"}]}}}]`)
	bindings, e := parseBindings(data)
	if e != nil || len(bindings) != 3 {
		t.Fatal(bindings, e)
	}
	ps := enrichBindings([]types.Process{{PID: 3, Name: "node", Port: 3000, Protocol: types.ProtocolTCP, Address: "127.0.0.3:3000"}, {PID: 4, Name: "com.docker.backend", Port: 3000, Protocol: types.ProtocolTCP, Address: "*:3000"}}, bindings)
	if ps[0].ContainerID != "" {
		t.Fatal("native node mislabeled as Docker")
	}
	found := map[string]bool{}
	for _, p := range ps {
		if p.ContainerID != "" {
			found[p.ContainerID] = true
		}
		if p.ContainerID == "one" && p.Cwd != "/work/app" {
			t.Fatal("compose ownership missing")
		}
	}
	if !found["one"] || !found["two"] {
		t.Fatal("shared backend lost container mappings", ps)
	}
	// Published UDP mappings also appear when Docker uses no userland proxy.
	udp := false
	for _, p := range ps {
		if p.Port == 5353 && p.Protocol == types.ProtocolUDP {
			udp = true
		}
	}
	if !udp {
		t.Fatal("proxyless port missing")
	}
}
