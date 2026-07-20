package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
	if cmd == nil {
		t.Error("expected a follow-up command to fetch artwork for the new track")
	}
}

func TestModel_Update_NowPlayingMsg_SameTrack_NoArtworkRefetch(t *testing.T) {
	track := &musicapp.Track{Name: "Song", Artist: "Artist", Album: "Album", State: "playing"}
	m := New(musicapp.NewClient(&stubRunner{}))
	m.track = track
	m.artwork = "cached-thumbnail"

	// Same identity, only the state field differs (paused vs playing) - this
	// must not trigger a refetch.
	got, cmd := m.Update(nowPlayingMsg{track: &musicapp.Track{Name: "Song", Artist: "Artist", Album: "Album", State: "paused"}})

	mm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if cmd != nil {
		t.Error("expected no follow-up command when the track identity is unchanged")
	}
	if mm.artwork != "cached-thumbnail" {
		t.Errorf("artwork = %q, want cached thumbnail to be left in place", mm.artwork)
	}
}

func TestModel_Update_NowPlayingMsg_TrackCleared_ClearsArtwork(t *testing.T) {
	m := New(musicapp.NewClient(&stubRunner{}))
	m.track = &musicapp.Track{Name: "Song"}
	m.artwork = "stale-thumbnail"

	got, cmd := m.Update(nowPlayingMsg{})

	mm, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", got)
	}
	if cmd != nil {
		t.Error("expected no follow-up command when nothing is playing")
	}
	if mm.artwork != "" {
		t.Errorf("artwork = %q, want cleared", mm.artwork)
	}
}

func TestModel_Update_ArtworkMsg(t *testing.T) {
	t.Run("success sets the rendered thumbnail", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{}))

		got, cmd := m.Update(artworkMsg{rendered: "thumbnail"})

		mm, ok := got.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", got)
		}
		if mm.artwork != "thumbnail" {
			t.Errorf("artwork = %q, want %q", mm.artwork, "thumbnail")
		}
		if cmd != nil {
			t.Error("expected no follow-up command")
		}
	})

	t.Run("error leaves the last known thumbnail in place", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{}))
		m.artwork = "old-thumbnail"

		got, _ := m.Update(artworkMsg{err: errors.New("boom")})

		mm, ok := got.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", got)
		}
		if mm.artwork != "old-thumbnail" {
			t.Errorf("artwork = %q, want unchanged %q", mm.artwork, "old-thumbnail")
		}
	})
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

// collectMsgs runs cmd and flattens any (possibly nested) tea.BatchMsg into the
// concrete messages its leaf commands produce.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, sub := range msg {
			out = append(out, collectMsgs(sub)...)
		}
	default:
		out = append(out, msg)
	}
	return out
}

func TestModel_Init_SchedulesTick(t *testing.T) {
	old := refreshInterval
	refreshInterval = time.Millisecond
	defer func() { refreshInterval = old }()

	m := New(musicapp.NewClient(&stubRunner{output: "stopped"}))

	var gotNowPlaying, gotTick bool
	for _, msg := range collectMsgs(m.Init()) {
		switch msg.(type) {
		case nowPlayingMsg:
			gotNowPlaying = true
		case tickMsg:
			gotTick = true
		}
	}
	if !gotNowPlaying {
		t.Error("expected Init to fetch now-playing state")
	}
	if !gotTick {
		t.Error("expected Init to schedule a tick")
	}
}

func TestModel_Update_TickMsg(t *testing.T) {
	old := refreshInterval
	refreshInterval = time.Millisecond
	defer func() { refreshInterval = old }()

	t.Run("refreshes state and reschedules the next tick", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{output: "stopped"}))

		_, cmd := m.Update(tickMsg(time.Now()))
		if cmd == nil {
			t.Fatal("expected a command")
		}

		var gotNowPlaying, gotTick bool
		for _, msg := range collectMsgs(cmd) {
			switch msg.(type) {
			case nowPlayingMsg:
				gotNowPlaying = true
			case tickMsg:
				gotTick = true
			}
		}
		if !gotNowPlaying {
			t.Error("expected the tick to refresh now-playing state")
		}
		if !gotTick {
			t.Error("expected the tick to reschedule itself")
		}
	})

	t.Run("stops ticking once quitting", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{}))
		m.quitting = true

		_, cmd := m.Update(tickMsg(time.Now()))
		if cmd != nil {
			t.Error("expected no command once quitting")
		}
	})
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
