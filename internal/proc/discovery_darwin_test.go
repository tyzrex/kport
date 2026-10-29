//go:build darwin

package proc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyzrex/kport/types"
)

func TestDarwinDiscoveryReportsFailureAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		body     string
		wantErr  bool
		deadline bool
	}{
		{"exit 1", false, false},
		{"echo broken >&2; exit 1", true, false},
		{"printf 'p123\\ncnode\\nn127.0.0.1:3000\\n'; exit 2", true, false},
		{"exec /bin/sleep 30", true, true},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "lsof"), []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		ctx := context.Background()
		cancel := func() {}
		if tc.deadline {
			ctx, cancel = context.WithTimeout(ctx, 40*time.Millisecond)
		}
		ps, err := FindAllProcesses(types.FindOptions{Context: ctx})
		cancel()
		if (err != nil) != tc.wantErr || len(ps) != 0 {
			t.Fatalf("ps=%v err=%v", ps, err)
		}
		if tc.deadline && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
}

func TestDarwinBatchesMetadataAcrossSockets(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	pid := os.Getpid()
	lsof := fmt.Sprintf(`#!/bin/sh
printf 'lsof\n' >> '%s'
case " $* " in
 *' cwd '*) printf 'p%d\nn/tmp/project\n' ;;
 *'-iUDP'*) exit 1 ;;
 *) printf 'p%d\ncnode\nn127.0.0.1:3000\nn127.0.0.1:3001\n' ;;
esac
`, log, pid, pid)
	ps := fmt.Sprintf(`#!/bin/sh
printf 'ps\n' >> '%s'
case " $* " in
 *'pid=,command='*) printf '%d node server.js\n' ;;
 *) printf '%d 1 1024 test /Applications/Test App/node\n' ;;
esac
`, log, pid, pid)
	for name, body := range map[string]string{"lsof": lsof, "ps": ps} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	got, err := FindAllProcesses(types.FindOptions{IncludeTree: true})
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	for _, p := range got {
		if p.Cwd != "/tmp/project" || p.CommandLine != "node server.js" || p.Exe != "/Applications/Test App/node" || p.StartID == "" {
			t.Fatal(p)
		}
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if count := len(strings.Fields(string(calls))); count != 5 {
		t.Fatalf("expected five batched commands, got %d: %s", count, calls)
	}
}
