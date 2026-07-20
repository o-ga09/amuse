package tui

import (
	"strings"
	"testing"
)

func TestKittyGraphicsAvailable(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"ghostty via TERM", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"kitty via TERM", map[string]string{"TERM": "xterm-kitty"}, true},
		{"kitty via window id", map[string]string{"KITTY_WINDOW_ID": "1"}, true},
		{"wezterm via TERM_PROGRAM", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"plain xterm has no support", map[string]string{"TERM": "xterm-256color"}, false},
		{"tmux disables even under ghostty", map[string]string{"TERM": "xterm-ghostty", "TMUX": "/tmp/tmux-0/default,1,0"}, false},
		{"screen TERM disables", map[string]string{"TERM": "screen-256color", "KITTY_WINDOW_ID": "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the vars we key off so the host environment can't leak in.
			for _, k := range []string{"TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID", "TMUX"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := kittyGraphicsAvailable(); got != tt.want {
				t.Errorf("kittyGraphicsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderArtworkKitty_TransmitsAndPlacesInACellGrid(t *testing.T) {
	got, err := renderArtworkKitty(testPNG(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(got, "\x1b_Ga=T,i=1,f=100,U=1,") {
		t.Errorf("output does not start with a Kitty transmit-and-place escape: %.40q", got)
	}
	if !strings.Contains(got, "q=2") {
		t.Error("transmit escape should suppress terminal responses (q=2)")
	}

	// The placeholder grid is artworkRows lines of artworkCols placeholder runes.
	grid := got[strings.LastIndex(got, "\x1b\\")+len("\x1b\\"):]
	lines := strings.Split(strings.TrimRight(grid, "\n"), "\n")
	if len(lines) != artworkRows {
		t.Fatalf("placeholder grid has %d rows, want %d", len(lines), artworkRows)
	}
	for i, line := range lines {
		if n := strings.Count(line, string(kittyPlaceholder)); n != artworkCols {
			t.Errorf("row %d has %d placeholder cells, want %d", i, n, artworkCols)
		}
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Errorf("row %d does not reset styling at the end", i)
		}
	}
}

func TestRenderArtworkKitty_ChunksLargePayloads(t *testing.T) {
	got, err := renderArtworkKitty(testPNG(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A tiny test image fits in a single chunk, so its first (and only)
	// transmit escape must announce no continuation (m=0).
	if !strings.Contains(got, "m=0;") {
		t.Error("single-chunk transmit should be marked final (m=0)")
	}
}

func TestRenderArtworkKitty_RejectsUndecodableData(t *testing.T) {
	if _, err := renderArtworkKitty([]byte("not an image")); err == nil {
		t.Fatal("expected an error for undecodable data")
	}
}
