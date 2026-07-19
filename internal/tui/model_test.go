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

	t.Run("success triggers a refresh", func(t *testing.T) {
		m := New(musicapp.NewClient(&stubRunner{output: "stopped"}))

		_, cmd := m.Update(actionMsg{})
		if cmd == nil {
			t.Fatal("expected a refresh command")
		}

		msg := cmd()
		if _, ok := msg.(nowPlayingMsg); !ok {
			t.Errorf("cmd produced %T, want nowPlayingMsg", msg)
		}
	})
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
