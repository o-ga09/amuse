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
