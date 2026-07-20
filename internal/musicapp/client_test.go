package musicapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"regexp"
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

// funcRunner lets a test observe (and act on) the exact script a call
// produces, which Artwork's tests need in order to drop bytes at the temp
// path the script embeds before reporting success.
type funcRunner struct {
	fn func(script string) (string, error)
}

func (f *funcRunner) Run(_ context.Context, script string) (string, error) {
	return f.fn(script)
}

var artworkPathPattern = regexp.MustCompile(`POSIX file "([^"]+)"`)

func extractArtworkPath(t *testing.T, script string) string {
	t.Helper()
	m := artworkPathPattern.FindStringSubmatch(script)
	if m == nil {
		t.Fatalf("script does not contain a POSIX file path: %q", script)
	}
	return m[1]
}

func TestClient_PlaybackActions(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*Client, context.Context) error
		wantScript string
	}{
		{"Play", (*Client).Play, "to play"},
		{"Pause", (*Client).Pause, "to pause"},
		{"Next", (*Client).Next, "next track"},
		{"Previous", (*Client).Previous, "previous track"},
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

func TestClient_NextAndPrevious_FallBackToPlayWhenStopped(t *testing.T) {
	tests := []struct {
		name         string
		call         func(*Client, context.Context) error
		wantWhenLive string
	}{
		{"Next", (*Client).Next, "next track"},
		{"Previous", (*Client).Previous, "previous track"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			c := NewClient(r)

			if err := tt.call(c, t.Context()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(r.script, "if player state is stopped") {
				t.Errorf("script should detect the stopped state: %q", r.script)
			}
			if !strings.Contains(r.script, "play") {
				t.Errorf("script should fall back to play when stopped: %q", r.script)
			}
			if !strings.Contains(r.script, tt.wantWhenLive) {
				t.Errorf("script should still %q while playing: %q", tt.wantWhenLive, r.script)
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

func TestClient_Search(t *testing.T) {
	t.Run("parses matching tracks", func(t *testing.T) {
		r := &fakeRunner{output: "Song A\tArtist A\tAlbum A\nSong B\tArtist B\tAlbum B"}
		c := NewClient(r)

		got, err := c.Search(t.Context(), "love")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []LibraryTrack{
			{Name: "Song A", Artist: "Artist A", Album: "Album A"},
			{Name: "Song B", Artist: "Artist B", Album: "Album B"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %d tracks, want %d: %+v", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("track %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("no matches returns nil", func(t *testing.T) {
		r := &fakeRunner{output: ""}
		c := NewClient(r)

		got, err := c.Search(t.Context(), "nothing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("escapes quotes and backslashes in the query", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if _, err := c.Search(t.Context(), `a" & (do shell script "x") & "b\`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `for "a\" & (do shell script \"x\") & \"b\\"`) {
			t.Errorf("query not escaped in script: %q", r.script)
		}
	})

	t.Run("drops control characters from the query", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if _, err := c.Search(t.Context(), "a\nb\tc"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `for "abc"`) {
			t.Errorf("control characters not dropped: %q", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if _, err := c.Search(t.Context(), "x"); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_Songs(t *testing.T) {
	t.Run("parses tracks and interpolates pagination", func(t *testing.T) {
		r := &fakeRunner{output: "Song\tArtist\tAlbum"}
		c := NewClient(r)

		got, err := c.Songs(t.Context(), 10, 20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []LibraryTrack{{Name: "Song", Artist: "Artist", Album: "Album"}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		if !strings.Contains(r.script, "set startIdx to 20 + 1") {
			t.Errorf("offset not interpolated: %q", r.script)
		}
		if !strings.Contains(r.script, "set lim to 10") {
			t.Errorf("limit not interpolated: %q", r.script)
		}
		// Guard against reintroducing the whole-library materialization that
		// SIGKILLed osascript on large libraries; the script must bound its read
		// to the requested window instead.
		if strings.Contains(r.script, "every track of library playlist 1") {
			t.Errorf("script materializes the whole library, want a bounded read: %q", r.script)
		}
	})

	t.Run("rejects a negative offset without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		_, err := c.Songs(t.Context(), 10, -1)
		if !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("err = %v, want %v", err, ErrInvalidPagination)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if _, err := c.Songs(t.Context(), 10, 0); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_Playlists(t *testing.T) {
	t.Run("parses newline-separated names", func(t *testing.T) {
		r := &fakeRunner{output: "My Chill Mix\nWorkout, Vol. 1\nFocus\n"}
		c := NewClient(r)

		got, err := c.Playlists(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"My Chill Mix", "Workout, Vol. 1", "Focus"}
		if len(got) != len(want) {
			t.Fatalf("got %d playlists, want %d: %+v", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("playlist %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("no playlists returns nil", func(t *testing.T) {
		r := &fakeRunner{output: ""}
		c := NewClient(r)

		got, err := c.Playlists(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if _, err := c.Playlists(t.Context()); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_PlaylistTracks(t *testing.T) {
	t.Run("parses tracks and escapes the playlist name", func(t *testing.T) {
		r := &fakeRunner{output: "Song\tArtist\tAlbum"}
		c := NewClient(r)

		got, err := c.PlaylistTracks(t.Context(), `Chill "Vibes"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != (LibraryTrack{Name: "Song", Artist: "Artist", Album: "Album"}) {
			t.Fatalf("got %+v, want a single Song/Artist/Album track", got)
		}
		if !strings.Contains(r.script, `name is "Chill \"Vibes\""`) {
			t.Errorf("playlist name not escaped in script: %q", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if _, err := c.PlaylistTracks(t.Context(), "x"); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_PlayPlaylist(t *testing.T) {
	t.Run("escapes the name and plays the playlist", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.PlayPlaylist(t.Context(), `a" & "b`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `play (first playlist whose name is "a\" & \"b")`) {
			t.Errorf("script = %q, want an escaped play-playlist command", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if err := c.PlayPlaylist(t.Context(), "x"); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_PlayPlaylistTrack(t *testing.T) {
	t.Run("interpolates a 1-based index and escapes the name", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.PlayPlaylistTrack(t.Context(), `My "List"`, 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `play (track 3 of (first playlist whose name is "My \"List\""))`) {
			t.Errorf("script = %q, want track 3 of the escaped playlist", r.script)
		}
	})

	t.Run("rejects a negative index without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.PlayPlaylistTrack(t.Context(), "x", -1); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("err = %v, want %v", err, ErrInvalidPagination)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})
}

func TestClient_PlaySong(t *testing.T) {
	t.Run("interpolates a 1-based library index", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.PlaySong(t.Context(), 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, "play (track 1 of library playlist 1)") {
			t.Errorf("script = %q, want track 1 of library playlist 1", r.script)
		}
	})

	t.Run("rejects a negative index without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.PlaySong(t.Context(), -1); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("err = %v, want %v", err, ErrInvalidPagination)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})
}

func TestClient_CreatePlaylist(t *testing.T) {
	t.Run("escapes the name and makes a new playlist", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.CreatePlaylist(t.Context(), `Road "Trip"`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `make new playlist with properties {name:"Road \"Trip\""}`) {
			t.Errorf("script = %q, want an escaped make-new-playlist command", r.script)
		}
	})

	t.Run("rejects a blank name without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.CreatePlaylist(t.Context(), "   "); !errors.Is(err, ErrEmptyPlaylistName) {
			t.Fatalf("err = %v, want %v", err, ErrEmptyPlaylistName)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if err := c.CreatePlaylist(t.Context(), "x"); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_DeletePlaylist(t *testing.T) {
	t.Run("escapes the name and deletes the playlist", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.DeletePlaylist(t.Context(), `My "List"`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(r.script, `delete (first playlist whose name is "My \"List\"")`) {
			t.Errorf("script = %q, want an escaped delete-playlist command", r.script)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if err := c.DeletePlaylist(t.Context(), "x"); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_AddSongToPlaylist(t *testing.T) {
	t.Run("interpolates a 1-based index and escapes the playlist name", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.AddSongToPlaylist(t.Context(), 4, `Fav "s"`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `duplicate (track 5 of library playlist 1) to (first playlist whose name is "Fav \"s\"")`
		if !strings.Contains(r.script, want) {
			t.Errorf("script = %q, want %q", r.script, want)
		}
	})

	t.Run("rejects a negative index without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.AddSongToPlaylist(t.Context(), -1, "x"); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("err = %v, want %v", err, ErrInvalidPagination)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})
}

func TestClient_RemovePlaylistTrack(t *testing.T) {
	t.Run("interpolates a 1-based index and escapes the playlist name", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.RemovePlaylistTrack(t.Context(), `My "List"`, 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `delete (track 3 of (first playlist whose name is "My \"List\""))`
		if !strings.Contains(r.script, want) {
			t.Errorf("script = %q, want %q", r.script, want)
		}
	})

	t.Run("rejects a negative index without running a script", func(t *testing.T) {
		r := &fakeRunner{}
		c := NewClient(r)

		if err := c.RemovePlaylistTrack(t.Context(), "x", -1); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("err = %v, want %v", err, ErrInvalidPagination)
		}
		if r.script != "" {
			t.Errorf("expected no script to run, got %q", r.script)
		}
	})
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

func TestClient_Artwork(t *testing.T) {
	t.Run("returns raw bytes and cleans up the temp file", func(t *testing.T) {
		want := []byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}
		var path string
		r := &funcRunner{fn: func(script string) (string, error) {
			path = extractArtworkPath(t, script)
			if err := os.WriteFile(path, want, 0o600); err != nil {
				t.Fatalf("write test artwork: %v", err)
			}
			return "ok", nil
		}}
		c := NewClient(r)

		got, err := c.Artwork(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("Artwork() = %v, want %v", got, want)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("expected temp file %q to be removed", path)
		}
	})

	t.Run("nothing playing", func(t *testing.T) {
		r := &fakeRunner{output: "stopped"}
		c := NewClient(r)

		if _, err := c.Artwork(t.Context()); !errors.Is(err, ErrNothingPlaying) {
			t.Errorf("err = %v, want %v", err, ErrNothingPlaying)
		}
	})

	t.Run("no artwork on track", func(t *testing.T) {
		r := &fakeRunner{output: "none"}
		c := NewClient(r)

		if _, err := c.Artwork(t.Context()); !errors.Is(err, ErrNoArtwork) {
			t.Errorf("err = %v, want %v", err, ErrNoArtwork)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		wantErr := errors.New("boom")
		r := &fakeRunner{err: wantErr}
		c := NewClient(r)

		if _, err := c.Artwork(t.Context()); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})

	t.Run("unexpected output", func(t *testing.T) {
		r := &fakeRunner{output: "huh"}
		c := NewClient(r)

		if _, err := c.Artwork(t.Context()); err == nil {
			t.Fatal("expected an error")
		}
	})
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
