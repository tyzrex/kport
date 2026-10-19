package detect

import (
	"github.com/tyzrex/kport/types"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectOwnershipDoesNotTrustDefaultPorts(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "apps", "web"), 0700)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"*"},"scripts":{"dev":"next dev --port 4100"}}`), 0600)
	info, ok := DetectProject(dir)
	if !ok || !reflect.DeepEqual(info.Ports, []int{4100}) {
		t.Fatal(info)
	}
	owned, suggested := MatchProject(dir, []types.Process{{PID: 2, Cwd: filepath.Join(dir, "apps", "web"), Port: 4200}, {PID: 3, Cwd: filepath.Join(dir, "..", "other"), Port: 4100}})
	if len(owned) != 1 || owned[0].PID != 2 || len(suggested) != 1 || suggested[0].PID != 3 {
		t.Fatalf("owned=%v suggested=%v", owned, suggested)
	}
	if owned[0].MatchReason == "" || suggested[0].MatchReason == "" {
		t.Fatal("missing match explanation")
	}
}
func TestComposeStructuredHostBindings(t *testing.T) {
	doc := `services:
  api:
    image: localhost:5000/myimage
    ports:
      - "127.0.0.1:8080:80"
      - "[::1]:8081:81/tcp"
      - target: 82
        published: "8082"
        protocol: udp
      - "9000-9002:9000-9002"
    environment:
      SAMPLE: "1234:5678"
`
	want := []int{8080, 8081, 8082, 9000, 9001, 9002}
	if got := parseComposePorts(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
func TestSymlinkProjectOwnership(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "alias")
	if e := os.Symlink(dir, link); e != nil {
		t.Skip(e)
	}
	if !within(link, dir) {
		t.Fatal("symlink should resolve to same project")
	}
	if within(dir, dir+"-other") {
		t.Fatal("sibling path matched")
	}
}

func TestWorkspaceServicesUseTheirOwnPorts(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "apps", "web"), 0700)
	os.MkdirAll(filepath.Join(dir, "apps", "api"), 0700)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"workspaces":["apps/*"]}`), 0600)
	os.WriteFile(filepath.Join(dir, "apps", "web", "package.json"), []byte(`{"scripts":{"dev":"vite --port 4100"}}`), 0600)
	os.WriteFile(filepath.Join(dir, "apps", "api", "package.json"), []byte(`{"scripts":{"dev":"PORT=4200 node server.js"}}`), 0600)
	info, ok := DetectProject(dir)
	if !ok || len(info.Services) != 2 || !reflect.DeepEqual(info.Ports, []int{4100, 4200}) {
		t.Fatalf("%+v", info)
	}
}
