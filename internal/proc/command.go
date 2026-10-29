package proc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Output bounds each OS query even when the caller supplied no deadline.
func Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return out, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

func lsofOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := Output(ctx, name, args...)
	var exit *exec.ExitError
	// lsof returns 1 without diagnostics when there are no matching files.
	if errors.As(err, &exit) && exit.ExitCode() == 1 && strings.TrimSpace(string(exit.Stderr)) == "" {
		return out, nil
	}
	return out, err
}
