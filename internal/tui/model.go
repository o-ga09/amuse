// Package tui provides an interactive terminal UI for controlling Music.app,
// built on top of internal/musicapp.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/o-ga09/amuse/internal/musicapp"
)

const actionTimeout = 5 * time.Second

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// nowPlayingMsg carries the result of a NowPlaying fetch. track is nil when
// nothing is playing; err is set only for genuine failures.
type nowPlayingMsg struct {
	track *musicapp.Track
	err   error
}

// actionMsg carries the result of a playback control action (play/pause/next/previous).
type actionMsg struct {
	err error
}

// Model is a Bubble Tea model that displays the current track and lets the
// user control playback with keybindings.
type Model struct {
	client   *musicapp.Client
	track    *musicapp.Track
	err      error
	quitting bool
}

// New creates a Model that controls Music.app through client.
func New(client *musicapp.Client) Model {
	return Model{client: client}
}

func (m Model) Init() tea.Cmd {
	return fetchNowPlaying(m.client)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case nowPlayingMsg:
		m.track = msg.track
		m.err = msg.err
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, fetchNowPlaying(m.client)
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("amuse") + "\n\n")

	switch {
	case m.err != nil:
		b.WriteString(errStyle.Render("error: "+m.err.Error()) + "\n\n")
	case m.track == nil:
		b.WriteString("Nothing playing.\n\n")
	default:
		fmt.Fprintf(&b, "%s — %s\n%s\n[%s]\n\n", m.track.Name, m.track.Artist, m.track.Album, m.track.State)
	}

	b.WriteString(dimStyle.Render("space: play/pause  n: next  p: prev  r: refresh  q: quit"))
	return b.String()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case " ":
		if m.track != nil && m.track.State == "playing" {
			return m, doAction(m.client.Pause)
		}
		return m, doAction(m.client.Play)
	case "n":
		return m, doAction(m.client.Next)
	case "p":
		return m, doAction(m.client.Previous)
	case "r":
		return m, fetchNowPlaying(m.client)
	}
	return m, nil
}

func fetchNowPlaying(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()

		track, err := c.NowPlaying(ctx)
		if err != nil {
			if errors.Is(err, musicapp.ErrNothingPlaying) {
				return nowPlayingMsg{}
			}
			return nowPlayingMsg{err: err}
		}
		return nowPlayingMsg{track: track}
	}
}

func doAction(action func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{err: action(ctx)}
	}
}

// Run starts the interactive TUI and blocks until the user quits.
func Run(client *musicapp.Client) error {
	_, err := tea.NewProgram(New(client)).Run()
	return err
}
