package tui

import (
	"testing"

	"github.com/o-ga09/amuse/internal/musicapp"
)

// These benchmarks cover the TUI render loop: Update (message handling) and
// View (string building). They exercise pure Go only — no osascript spawns,
// since the commands Update returns are never run here. See issue #13.

func benchModel() Model {
	m := New(musicapp.NewClient(&stubRunner{}))
	m.track = &musicapp.Track{Name: "Song", Artist: "Artist", Album: "Album", State: "playing"}
	m.shuffle = true
	m.repeat = musicapp.RepeatAll
	m.volume = 50
	return m
}

func BenchmarkModel_Update_NowPlaying(b *testing.B) {
	m := benchModel()
	msg := nowPlayingMsg{track: &musicapp.Track{Name: "Song", Artist: "Artist", Album: "Album", State: "playing"}}

	b.ReportAllocs()
	for b.Loop() {
		m.Update(msg)
	}
}

func BenchmarkModel_View(b *testing.B) {
	m := benchModel()

	b.ReportAllocs()
	for b.Loop() {
		_ = m.View()
	}
}
