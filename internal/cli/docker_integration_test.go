//go:build !windows

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The fake engine acknowledges stop but deliberately leaves a listener alive.
// This exercises the real CLI's release timeout and JSON/quiet exit statuses.
func TestDockerWaitTimeoutFailsInJSONAndQuietModes(t *testing.T) {
	dir := t.TempDir()
	_, port := listener(t, dir)
	bindir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
 ps) echo fake-container ;;
 inspect) echo '[{"Id":"fake-container","Name":"/test","NetworkSettings":{"Ports":{"80/tcp":[{"HostIP":"127.0.0.1","HostPort":"%d"}]}}}]' ;;
 stop|kill) exit 0 ;;
 *) exit 1 ;;
esac
`, port)
	if e := os.WriteFile(filepath.Join(bindir, "docker"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"--json", "--quiet"} {
		cmd := command(t, dir, strconv.Itoa(port), "-W", "--wait-timeout", "30ms", mode, "--name", "docker:test")
		cmd.Env = append(cmd.Env, "PATH="+bindir+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, e := cmd.Output()
		if e == nil {
			t.Fatalf("%s returned success while port remained occupied: %s", mode, out)
		}
		if mode == "--json" {
			var rows []struct{ Error string }
			if e = json.Unmarshal(out, &rows); e != nil || len(rows) == 0 || !strings.Contains(rows[0].Error, "timeout") {
				t.Fatalf("invalid failure JSON: %s (%v)", out, e)
			}
		} else if len(out) > 0 {
			t.Fatalf("quiet wrote stdout: %s", out)
		}
		assertListening(t, port)
	}
}
