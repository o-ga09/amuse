package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/o-ga09/amuse/internal/musicapp"
)

// fakeRunner is a musicapp.Runner that returns canned output without touching
// osascript, and records what it was asked to run so tests can assert whether
// (and with what script) the client was actually invoked.
type fakeRunner struct {
	output string
	err    error

	calls  int
	script string
}

func (f *fakeRunner) Run(_ context.Context, script string) (string, error) {
	f.calls++
	f.script = script
	return f.output, f.err
}

// execute runs the root command with args, its client backed by r, and returns
// everything written to the command's output stream plus the Execute error.
func execute(t *testing.T, r musicapp.Runner, args ...string) (string, error) {
	t.Helper()

	orig := newClient
	newClient = func() *musicapp.Client { return musicapp.NewClient(r) }
	t.Cleanup(func() { newClient = orig })

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	return buf.String(), err
}
