//go:build !windows

package proc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLsofExitStatuses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		fail       bool
		output     string
	}{
		{"empty", "exit 1", false, ""},
		{"partial-selection", "printf 'p123\\n'; exit 1", false, "p123\n"},
		{"diagnostic", "echo permission-denied >&2; exit 1", true, ""},
		{"failure-with-output", "printf 'p123\\n'; exit 2", true, "p123\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "lsof")
			if err := os.WriteFile(file, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			out, err := lsofOutput(context.Background(), file)
			if (err != nil) != tc.fail || string(out) != tc.output {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestOutputCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Output(ctx, "sh", "-c", "exec sleep 30")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not stop query promptly")
	}
}
