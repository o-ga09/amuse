package cmd

import (
	"errors"
	"strings"
	"testing"
)

// errBoom stands in for any failure the underlying runner surfaces, so tests
// can assert a command propagates it rather than swallowing it.
var errBoom = errors.New("boom")

func TestPlaybackCommands_InvokeClientAndPropagateErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"play", []string{"play"}},
		{"pause", []string{"pause"}},
		{"next", []string{"next"}},
		{"prev", []string{"prev"}},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Succeeds", func(t *testing.T) {
			r := &fakeRunner{}
			out, err := execute(t, r, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.calls != 1 {
				t.Errorf("runner called %d times, want 1", r.calls)
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
		})

		t.Run(tt.name+"_PropagatesError", func(t *testing.T) {
			out, err := execute(t, &fakeRunner{err: errBoom}, tt.args...)
			if !errors.Is(err, errBoom) {
				t.Fatalf("error = %v, want %v", err, errBoom)
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
		})
	}
}

func TestNow_PrintsTrack(t *testing.T) {
	r := &fakeRunner{output: "Song\nArtist\nAlbum\nplaying"}
	out, err := execute(t, r, "now")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Song — Artist\nAlbum: Album\nState: playing\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestNow_NothingPlaying(t *testing.T) {
	r := &fakeRunner{output: "stopped"}
	out, err := execute(t, r, "now")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Nothing playing.\n" {
		t.Errorf("output = %q, want %q", out, "Nothing playing.\n")
	}
}

func TestNow_PropagatesError(t *testing.T) {
	_, err := execute(t, &fakeRunner{err: errBoom}, "now")
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}

func TestShuffle_Get(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{"enabled", "true", "on\n"},
		{"disabled", "false", "off\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := execute(t, &fakeRunner{output: tt.output}, "shuffle")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tt.want {
				t.Errorf("output = %q, want %q", out, tt.want)
			}
		})
	}
}

func TestShuffle_Set(t *testing.T) {
	tests := []struct {
		arg      string
		wantTrue bool
	}{
		{"on", true},
		{"off", false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			r := &fakeRunner{}
			out, err := execute(t, r, "shuffle", tt.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
			if got := strings.Contains(r.script, "to true"); got != tt.wantTrue {
				t.Errorf("script %q set shuffle true=%v, want %v", r.script, got, tt.wantTrue)
			}
		})
	}
}

func TestShuffle_InvalidArgErrorsBeforeClient(t *testing.T) {
	r := &fakeRunner{}
	_, err := execute(t, r, "shuffle", "maybe")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if r.calls != 0 {
		t.Errorf("runner called %d times, want 0 (validation must precede the client)", r.calls)
	}
}

func TestRepeat_Get(t *testing.T) {
	out, err := execute(t, &fakeRunner{output: "one"}, "repeat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "one\n" {
		t.Errorf("output = %q, want %q", out, "one\n")
	}
}

func TestRepeat_Set(t *testing.T) {
	r := &fakeRunner{}
	out, err := execute(t, r, "repeat", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("unexpected output %q", out)
	}
	if !strings.Contains(r.script, "set song repeat to all") {
		t.Errorf("script = %q, want it to set repeat to all", r.script)
	}
}

func TestRepeat_InvalidModeErrors(t *testing.T) {
	r := &fakeRunner{}
	_, err := execute(t, r, "repeat", "sometimes")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "off|one|all") {
		t.Errorf("error = %q, want it to mention the valid modes", err)
	}
	if r.calls != 0 {
		t.Errorf("runner called %d times, want 0", r.calls)
	}
}

func TestVolume_Get(t *testing.T) {
	out, err := execute(t, &fakeRunner{output: "42"}, "volume")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "42\n" {
		t.Errorf("output = %q, want %q", out, "42\n")
	}
}

func TestVolume_Set(t *testing.T) {
	r := &fakeRunner{}
	out, err := execute(t, r, "volume", "30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("unexpected output %q", out)
	}
	if !strings.Contains(r.script, "set sound volume to 30") {
		t.Errorf("script = %q, want it to set volume to 30", r.script)
	}
}

func TestVolume_NonIntegerErrorsBeforeClient(t *testing.T) {
	r := &fakeRunner{}
	_, err := execute(t, r, "volume", "abc")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if r.calls != 0 {
		t.Errorf("runner called %d times, want 0 (parsing must precede the client)", r.calls)
	}
}

func TestSearch_PrintsTabSeparatedTracks(t *testing.T) {
	r := &fakeRunner{output: "Song\tArtist\tAlbum\n"}
	out, err := execute(t, r, "search", "hello", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Song\tArtist\tAlbum\n" {
		t.Errorf("output = %q, want %q", out, "Song\tArtist\tAlbum\n")
	}
	// Multi-word queries are joined with a space before searching.
	if !strings.Contains(r.script, "hello world") {
		t.Errorf("script = %q, want it to search for the joined query", r.script)
	}
}

func TestSearch_RequiresQuery(t *testing.T) {
	r := &fakeRunner{}
	_, err := execute(t, r, "search")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if r.calls != 0 {
		t.Errorf("runner called %d times, want 0", r.calls)
	}
}

func TestSongs_PrintsTracks(t *testing.T) {
	r := &fakeRunner{output: "Song\tArtist\tAlbum\n"}
	out, err := execute(t, r, "songs", "--limit", "10", "--offset", "0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Song\tArtist\tAlbum\n" {
		t.Errorf("output = %q, want %q", out, "Song\tArtist\tAlbum\n")
	}
}

func TestSongs_NegativeOffsetErrorsBeforeClient(t *testing.T) {
	r := &fakeRunner{}
	_, err := execute(t, r, "songs", "--offset", "-1")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if r.calls != 0 {
		t.Errorf("runner called %d times, want 0", r.calls)
	}
}
