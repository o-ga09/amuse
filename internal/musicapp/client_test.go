package musicapp

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	output string
	err    error
	script string
}

func (f *fakeRunner) Run(_ context.Context, script string) (string, error) {
	f.script = script
	return f.output, f.err
}

func TestClient_PlaybackActions(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*Client, context.Context) error
		wantScript string
	}{
		{"Play", (*Client).Play, "to play"},
		{"Pause", (*Client).Pause, "to pause"},
		{"Next", (*Client).Next, "to next track"},
		{"Previous", (*Client).Previous, "to previous track"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_RunsExpectedScript", func(t *testing.T) {
			r := &fakeRunner{}
			c := NewClient(r)

			if err := tt.call(c, t.Context()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})

		t.Run(tt.name+"_PropagatesRunnerError", func(t *testing.T) {
			wantErr := errors.New("boom")
			r := &fakeRunner{err: wantErr}
			c := NewClient(r)

			if err := tt.call(c, t.Context()); !errors.Is(err, wantErr) {
				t.Errorf("err = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestClient_NowPlaying(t *testing.T) {
	wantRunnerErr := errors.New("boom")

	tests := []struct {
		name       string
		output     string
		runnerErr  error
		want       *Track
		wantErr    error
		wantErrAny bool
	}{
		{
			name:   "playing track",
			output: "Song\nArtist\nAlbum\nplaying",
			want:   &Track{Name: "Song", Artist: "Artist", Album: "Album", State: "playing"},
		},
		{
			name:    "stopped",
			output:  "stopped",
			wantErr: ErrNothingPlaying,
		},
		{
			name:       "malformed output",
			output:     "only one field",
			wantErrAny: true,
		},
		{
			name:      "runner error",
			runnerErr: wantRunnerErr,
			wantErr:   wantRunnerErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{output: tt.output, err: tt.runnerErr}
			c := NewClient(r)

			got, err := c.NowPlaying(t.Context())

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrAny:
				if err == nil {
					t.Fatal("expected an error")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if *got != *tt.want {
					t.Errorf("got %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

func TestClient_Shuffle(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"enabled", "true", true},
		{"disabled", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{output: tt.output}
			c := NewClient(r)

			got, err := c.Shuffle(t.Context())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Shuffle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_SetShuffle(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		wantScript string
	}{
		{"on", true, "shuffle enabled to true"},
		{"off", false, "shuffle enabled to false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			c := NewClient(r)

			if err := c.SetShuffle(t.Context(), tt.enabled); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestClient_Repeat(t *testing.T) {
	r := &fakeRunner{output: "all"}
	c := NewClient(r)

	got, err := c.Repeat(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != RepeatAll {
		t.Errorf("Repeat() = %q, want %q", got, RepeatAll)
	}
}

func TestClient_SetRepeat(t *testing.T) {
	tests := []struct {
		name       string
		mode       RepeatMode
		wantScript string
		wantErr    error
	}{
		{name: "off", mode: RepeatOff, wantScript: "song repeat to off"},
		{name: "one", mode: RepeatOne, wantScript: "song repeat to one"},
		{name: "all", mode: RepeatAll, wantScript: "song repeat to all"},
		{name: "invalid", mode: RepeatMode("bogus"), wantErr: ErrInvalidRepeatMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			c := NewClient(r)

			err := c.SetRepeat(t.Context(), tt.mode)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if r.script != "" {
					t.Errorf("expected no script to run for an invalid mode, got %q", r.script)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestClient_Volume(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		want       int
		wantErrAny bool
	}{
		{name: "typical level", output: "50", want: 50},
		{name: "non-numeric output", output: "not-a-number", wantErrAny: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{output: tt.output}
			c := NewClient(r)

			got, err := c.Volume(t.Context())

			if tt.wantErrAny {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Volume() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClient_SetVolume(t *testing.T) {
	tests := []struct {
		name       string
		level      int
		wantScript string
		wantErr    error
	}{
		{name: "mid range", level: 50, wantScript: "sound volume to 50"},
		{name: "zero", level: 0, wantScript: "sound volume to 0"},
		{name: "max", level: 100, wantScript: "sound volume to 100"},
		{name: "too low", level: -1, wantErr: ErrInvalidVolume},
		{name: "too high", level: 101, wantErr: ErrInvalidVolume},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			c := NewClient(r)

			err := c.SetVolume(t.Context(), tt.level)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if r.script != "" {
					t.Errorf("expected no script to run for an out-of-range level, got %q", r.script)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}
