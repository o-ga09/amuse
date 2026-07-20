package musicapp

import (
	"context"
	"os/exec"
	"testing"
)

// These benchmarks isolate the pure-Go cost of Client methods (script
// construction + output parsing) by running them against a fake Runner, so the
// numbers exclude the osascript process spawn entirely. Compare them with
// BenchmarkOSARunner_RoundTrip below to see how little of a command's total
// latency is Go code. See issue #13.

func BenchmarkClient_NowPlaying(b *testing.B) {
	c := NewClient(&fakeRunner{output: "Song\nArtist\nAlbum\nplaying"})
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := c.NowPlaying(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClient_Play(b *testing.B) {
	c := NewClient(&fakeRunner{})
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if err := c.Play(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClient_SetVolume(b *testing.B) {
	c := NewClient(&fakeRunner{})
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if err := c.SetVolume(ctx, 50); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOSARunner_RoundTrip measures a real osascript spawn+exec+teardown so
// the process-spawn cost can be compared against the pure-Go Client benchmarks.
// It runs a trivial script that doesn't touch Music.app (so it's safe on any
// macOS box), and skips when osascript isn't on PATH (e.g. CI on Linux) or in
// -short mode, since spawning a process per iteration is slow.
func BenchmarkOSARunner_RoundTrip(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping osascript round-trip benchmark in -short mode")
	}
	if _, err := exec.LookPath("osascript"); err != nil {
		b.Skipf("osascript not available: %v", err)
	}

	r := OSARunner{}
	ctx := context.Background()
	const script = `return "ok"`

	b.ReportAllocs()
	for b.Loop() {
		if _, err := r.Run(ctx, script); err != nil {
			b.Fatal(err)
		}
	}
}
