package musicapp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes an AppleScript and returns its trimmed stdout.
type Runner interface {
	Run(ctx context.Context, script string) (string, error)
}

// OSARunner runs AppleScript via the macOS `osascript` binary. The script is
// passed as a single argument (never through a shell), so no argument built
// from external input can be interpreted as a separate shell command.
type OSARunner struct{}

func (OSARunner) Run(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
