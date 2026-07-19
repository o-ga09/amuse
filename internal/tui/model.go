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

// actionMsg carries the result of a playback control action (play/pause/next/previous/...).
type actionMsg struct {
	err error
}

// shuffleMsg, repeatMsg, and volumeMsg carry the result of their respective
// status fetches. A fetch error just leaves the last known value on screen
// rather than displacing the (more important) track/action error, so err is
// only inspected inside Update, never surfaced to the user directly.
type shuffleMsg struct {
	enabled bool
	err     error
}

type repeatMsg struct {
	mode musicapp.RepeatMode
	err  error
}

type volumeMsg struct {
	level int
	err   error
}

// Model is a Bubble Tea model that displays the current track and lets the
// user control playback with keybindings.
type Model struct {
	client   *musicapp.Client
	track    *musicapp.Track
	shuffle  bool
	repeat   musicapp.RepeatMode
	volume   int
	err      error
	quitting bool
}

// New creates a Model that controls Music.app through client.
func New(client *musicapp.Client) Model {
	return Model{client: client}
}

func (m Model) Init() tea.Cmd {
	return fetchAll(m.client)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case nowPlayingMsg:
		m.track = msg.track
		m.err = msg.err
		return m, nil
	case shuffleMsg:
		if msg.err == nil {
			m.shuffle = msg.enabled
		}
		return m, nil
	case repeatMsg:
		if msg.err == nil {
			m.repeat = msg.mode
		}
		return m, nil
	case volumeMsg:
		if msg.err == nil {
			m.volume = msg.level
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, fetchAll(m.client)
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

	fmt.Fprintf(&b, "shuffle: %s  repeat: %s  volume: %d\n\n", onOff(m.shuffle), m.repeat, m.volume)
	b.WriteString(dimStyle.Render("space: play/pause  n: next  p: prev  s: shuffle  c: repeat  +/-: volume  r: refresh  q: quit"))
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
	case "s":
		enable := !m.shuffle
		return m, doAction(func(ctx context.Context) error {
			return m.client.SetShuffle(ctx, enable)
		})
	case "c":
		next := nextRepeatMode(m.repeat)
		return m, doAction(func(ctx context.Context) error {
			return m.client.SetRepeat(ctx, next)
		})
	case "+", "=":
		level := clamp(m.volume+5, 0, 100)
		return m, doAction(func(ctx context.Context) error {
			return m.client.SetVolume(ctx, level)
		})
	case "-", "_":
		level := clamp(m.volume-5, 0, 100)
		return m, doAction(func(ctx context.Context) error {
			return m.client.SetVolume(ctx, level)
		})
	case "r":
		return m, fetchAll(m.client)
	}
	return m, nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func nextRepeatMode(mode musicapp.RepeatMode) musicapp.RepeatMode {
	switch mode {
	case musicapp.RepeatOff:
		return musicapp.RepeatAll
	case musicapp.RepeatAll:
		return musicapp.RepeatOne
	default:
		return musicapp.RepeatOff
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

func fetchShuffle(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		enabled, err := c.Shuffle(ctx)
		return shuffleMsg{enabled: enabled, err: err}
	}
}

func fetchRepeat(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		mode, err := c.Repeat(ctx)
		return repeatMsg{mode: mode, err: err}
	}
}

func fetchVolume(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		level, err := c.Volume(ctx)
		return volumeMsg{level: level, err: err}
	}
}

// fetchAll refreshes track info and shuffle/repeat/volume status in
// parallel; each fetch is an independent osascript invocation.
func fetchAll(c *musicapp.Client) tea.Cmd {
	return tea.Batch(fetchNowPlaying(c), fetchShuffle(c), fetchRepeat(c), fetchVolume(c))
}

func doAction(action func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionMsg{err: action(ctx)}
	}
}

// Run prints the startup banner, then starts the interactive TUI and blocks
// until the user quits.
func Run(client *musicapp.Client) error {
	fmt.Println(banner())
	_, err := tea.NewProgram(New(client)).Run()
	return err
}
