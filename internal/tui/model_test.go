package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/o-ga09/amuse/internal/musicapp"
)

type stubRunner struct {
	output string
	err    error
	script string
}

func (s *stubRunner) Run(_ context.Context, script string) (string, error) {
	s.script = script
	return s.output, s.err
}

func TestModel_Update_Quit(t *testing.T) {
	m := New(musicapp.NewClient(&stubRunner{}))

	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	mm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if !mm.quitting {
		t.Error("quitting = false, want true")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Quit command")
	}
}

func TestModel_Update_SpaceTogglesPlayPause(t *testing.T) {
	tests := []struct {
		name       string
		track      *musicapp.Track
		wantScript string
	}{
		{"nothing playing starts playback", nil, "to play"},
		{"playing pauses", &musicapp.Track{State: "playing"}, "to pause"},
		{"paused resumes", &musicapp.Track{State: "paused"}, "to play"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &stubRunner{}
			m := New(musicapp.NewClient(r))
			m.track = tt.track

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
			if cmd == nil {
				t.Fatal("expected a command")
			}
			cmd()

			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestModel_Update_NowPlayingMsg_SetsTrack(t *testing.T) {
	m := New(musicapp.NewClient(&stubRunner{}))
	track := &musicapp.Track{Name: "Song"}

	got, cmd := m.Update(nowPlayingMsg{track: track})

	mm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if mm.track != track {
		t.Errorf("track = %+v, want %+v", mm.track, track)
	}
	if cmd != nil {
		t.Error("expected no follow-up command")
	}
}

func TestModel_Update_ActionMsg(t *testing.T) {
	t.Run("error sets err and does not refresh", func(t *testing.T) {
		wantErr := errors.New("boom")
		m := New(musicapp.NewClient(&stubRunner{}))

		got, cmd := m.Update(actionMsg{err: wantErr})

		mm, ok := got.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", got)
		}
		if !errors.Is(mm.err, wantErr) {
			t.Errorf("err = %v, want %v", mm.err, wantErr)
		}
		if cmd != nil {
			t.Error("expected no follow-up command on error")
		}
	})

	t.Run("success triggers a refresh of everything", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{output: "stopped"}))

		_, cmd := m.Update(actionMsg{})
		if cmd == nil {
			t.Fatal("expected a refresh command")
		}

		batch, ok := cmd().(tea.BatchMsg)
		if !ok {
			t.Fatalf("cmd produced %T, want tea.BatchMsg", cmd())
		}

		var gotNowPlaying bool
		for _, sub := range batch {
			if _, ok := sub().(nowPlayingMsg); ok {
				gotNowPlaying = true
			}
		}
		if !gotNowPlaying {
			t.Error("expected the batch to include a nowPlayingMsg-producing command")
		}
	})
}

func TestModel_Update_ShuffleKey(t *testing.T) {
	tests := []struct {
		name       string
		shuffle    bool
		wantScript string
	}{
		{"off enables", false, "shuffle enabled to true"},
		{"on disables", true, "shuffle enabled to false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &stubRunner{}
			m := New(musicapp.NewClient(r))
			m.shuffle = tt.shuffle

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
			if cmd == nil {
				t.Fatal("expected a command")
			}
			cmd()

			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestModel_Update_RepeatKey(t *testing.T) {
	tests := []struct {
		name       string
		repeat     musicapp.RepeatMode
		wantScript string
	}{
		{"off cycles to all", musicapp.RepeatOff, "song repeat to all"},
		{"all cycles to one", musicapp.RepeatAll, "song repeat to one"},
		{"one cycles to off", musicapp.RepeatOne, "song repeat to off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &stubRunner{}
			m := New(musicapp.NewClient(r))
			m.repeat = tt.repeat

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
			if cmd == nil {
				t.Fatal("expected a command")
			}
			cmd()

			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestModel_Update_VolumeKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		volume     int
		wantScript string
	}{
		{"increase", "+", 50, "sound volume to 55"},
		{"decrease", "-", 50, "sound volume to 45"},
		{"increase clamps at 100", "+", 98, "sound volume to 100"},
		{"decrease clamps at 0", "-", 2, "sound volume to 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &stubRunner{}
			m := New(musicapp.NewClient(r))
			m.volume = tt.volume

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			if cmd == nil {
				t.Fatal("expected a command")
			}
			cmd()

			if !strings.Contains(r.script, tt.wantScript) {
				t.Errorf("script = %q, want substring %q", r.script, tt.wantScript)
			}
		})
	}
}

func TestModel_Update_StatusMsgs(t *testing.T) {
	m := New(musicapp.NewClient(&stubRunner{}))

	got, cmd := m.Update(shuffleMsg{enabled: true})
	mm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if !mm.shuffle {
		t.Error("shuffle = false, want true")
	}
	if cmd != nil {
		t.Error("expected no follow-up command")
	}

	got, _ = mm.Update(repeatMsg{mode: musicapp.RepeatAll})
	mm, ok = got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if mm.repeat != musicapp.RepeatAll {
		t.Errorf("repeat = %q, want %q", mm.repeat, musicapp.RepeatAll)
	}

	got, _ = mm.Update(volumeMsg{level: 42})
	mm, ok = got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if mm.volume != 42 {
		t.Errorf("volume = %d, want 42", mm.volume)
	}

	// An error on a status fetch leaves the last known value alone.
	got, _ = mm.Update(volumeMsg{level: 0, err: errors.New("boom")})
	mm, ok = got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if mm.volume != 42 {
		t.Errorf("volume = %d after failed fetch, want unchanged 42", mm.volume)
	}
}

func TestNextRepeatMode(t *testing.T) {
	tests := []struct {
		in   musicapp.RepeatMode
		want musicapp.RepeatMode
	}{
		{musicapp.RepeatOff, musicapp.RepeatAll},
		{musicapp.RepeatAll, musicapp.RepeatOne},
		{musicapp.RepeatOne, musicapp.RepeatOff},
	}

	for _, tt := range tests {
		if got := nextRepeatMode(tt.in); got != tt.want {
			t.Errorf("nextRepeatMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, lo, hi, want int
	}{
		{50, 0, 100, 50},
		{-5, 0, 100, 0},
		{105, 0, 100, 100},
	}

	for _, tt := range tests {
		if got := clamp(tt.v, tt.lo, tt.hi); got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

func TestModel_View(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want string
	}{
		{
			name: "nothing playing",
			m:    Model{},
			want: "Nothing playing.",
		},
		{
			name: "error",
			m:    Model{err: errors.New("boom")},
			want: "error: boom",
		},
		{
			name: "playing track",
			m:    Model{track: &musicapp.Track{Name: "Song", Artist: "Artist", Album: "Album", State: "playing"}},
			want: "Song",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.View(); !strings.Contains(got, tt.want) {
				t.Errorf("View() = %q, want substring %q", got, tt.want)
			}
		})
	}
}
