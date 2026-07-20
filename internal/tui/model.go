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

// listTimeout is longer than actionTimeout because listing playlists or a page
// of library tracks reads across the Apple event boundary and can take several
// seconds on a large library; a 5s cap SIGKILLs osascript mid-read.
const listTimeout = 30 * time.Second

// refreshInterval is how often the TUI polls Music.app so that changes made
// outside the TUI (auto-advance to the next track, or playback controlled from
// Music.app directly) show up without user input. Each tick spawns a fresh set
// of osascript processes, so keep it from tightening below the ~1s range. It's
// a var rather than a const so tests can shorten it.
var refreshInterval = time.Second

// brandColor is the "amuse" wordmark's magenta (see banner.go); the active tab
// picks it up so the selected tab reads as part of the same brand.
const brandColor = lipgloss.Color("212")

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	// activeTabStyle fills the selected tab with the brand color; black text
	// keeps it readable on the light magenta background.
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(brandColor).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	cursorStyle      = lipgloss.NewStyle().Bold(true).Foreground(brandColor)
)

// listVisibleRows caps how many list entries are shown at once; the cursor
// scrolls a window of this size through longer lists.
const listVisibleRows = 12

// tab identifies one of the top-level views the user cycles through with
// tab/shift+tab.
type tab int

const (
	tabNowPlaying tab = iota
	tabPlaylists
	tabSongs
	tabCount
)

// inputMode is modeNone during normal browsing; the other modes overlay a
// prompt on the browser to drive a playlist-management action (issue #28): a
// text prompt for a new name, a y/n confirm for a destructive action, or a
// target-playlist picker.
type inputMode int

const (
	modeNone inputMode = iota
	modeCreatePlaylist
	modeConfirmDeletePlaylist
	modeConfirmRemoveTrack
	modeAddToPlaylist
)

func (t tab) title() string {
	switch t {
	case tabPlaylists:
		return "Playlists"
	case tabSongs:
		return "Songs"
	default:
		return "Now Playing"
	}
}

// Library-browsing message types. playlistsMsg and songsMsg carry the result of
// listing the library; playlistTracksMsg carries the tracks of a drilled-into
// playlist (playlist echoes which one, so a stale response can be ignored).
type playlistsMsg struct {
	playlists []string
	err       error
}

type playlistTracksMsg struct {
	playlist string
	tracks   []musicapp.LibraryTrack
	err      error
}

type songsMsg struct {
	songs []musicapp.LibraryTrack
	err   error
}

// songsPageSize bounds how many library tracks the Songs tab loads; the whole
// library can be tens of thousands of tracks, and PlaySong addresses them by
// their 0-based position in this same (offset 0) ordering.
const songsPageSize = 500

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

// libraryActionMsg carries the result of a playlist-mutating action (issue #28)
// that has no list of its own to reload — currently only adding a song to a
// playlist. Create/delete/remove instead reload their affected list by
// returning a playlistsMsg/playlistTracksMsg on success. err populates listErr.
type libraryActionMsg struct {
	err error
}

// artworkMsg carries the result of an artwork fetch+render. Like
// shuffle/repeat/volume, a fetch error just leaves the last-shown thumbnail
// in place rather than displacing the track/action error.
type artworkMsg struct {
	rendered string
	err      error
}

// tickMsg is delivered on refreshInterval to drive periodic polling of
// Music.app; see refreshInterval.
type tickMsg time.Time

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
	artwork  string
	shuffle  bool
	repeat   musicapp.RepeatMode
	volume   int
	err      error
	quitting bool

	// Library browsing.
	tab             tab
	playlists       []string
	playlistsLoaded bool
	playlistCursor  int
	// openPlaylist is "" while the Playlists tab shows the playlist list, and
	// the selected playlist's name once the user has drilled into its tracks.
	openPlaylist   string
	playlistTracks []musicapp.LibraryTrack
	trackCursor    int
	songs          []musicapp.LibraryTrack
	songsLoaded    bool
	songCursor     int
	// listErr holds a browsing-side failure without displacing the now-playing
	// track/action error.
	listErr error

	// Playlist management overlays (issue #28). mode is modeNone during normal
	// browsing; the fields below back the active overlay.
	mode          inputMode
	nameInput     string // buffer for the create-playlist text prompt
	confirmTarget string // playlist name shown in the delete-playlist confirm
	addSongIndex  int    // library index of the song being added to a playlist
	pickerCursor  int    // cursor within the add-to-playlist target picker
}

// New creates a Model that controls Music.app through client.
func New(client *musicapp.Client) Model {
	return Model{client: client}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchAll(m.client), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tickMsg:
		if m.quitting {
			return m, nil
		}
		return m, tea.Batch(fetchAll(m.client), tick())
	case nowPlayingMsg:
		trackChanged := trackIdentity(m.track) != trackIdentity(msg.track)
		m.track = msg.track
		m.err = msg.err
		if !trackChanged {
			return m, nil
		}
		if msg.track == nil {
			m.artwork = ""
			return m, nil
		}
		return m, fetchArtwork(m.client)
	case artworkMsg:
		if msg.err == nil {
			m.artwork = msg.rendered
		}
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
	case libraryActionMsg:
		m.listErr = msg.err
		return m, nil
	case playlistsMsg:
		m.playlistsLoaded = true
		m.listErr = msg.err
		if msg.err == nil {
			m.playlists = msg.playlists
			m.playlistCursor = clamp(m.playlistCursor, 0, maxCursor(len(m.playlists)))
		}
		return m, nil
	case playlistTracksMsg:
		// Ignore a response for a playlist the user has since navigated away from.
		if msg.playlist != m.openPlaylist {
			return m, nil
		}
		m.listErr = msg.err
		if msg.err == nil {
			m.playlistTracks = msg.tracks
			m.trackCursor = clamp(m.trackCursor, 0, maxCursor(len(m.playlistTracks)))
		}
		return m, nil
	case songsMsg:
		m.songsLoaded = true
		m.listErr = msg.err
		if msg.err == nil {
			m.songs = msg.songs
			m.songCursor = clamp(m.songCursor, 0, maxCursor(len(m.songs)))
		}
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("amuse") + "\n")
	b.WriteString(m.renderTabBar() + "\n\n")

	switch m.tab {
	case tabPlaylists:
		m.renderPlaylistsTab(&b)
	case tabSongs:
		m.renderSongsTab(&b)
	default:
		m.renderNowPlayingTab(&b)
	}

	if m.mode != modeNone {
		m.renderMode(&b)
	}

	b.WriteString("\n" + dimStyle.Render(m.helpLine()))
	return b.String()
}

// renderMode draws the active playlist-management overlay below the tab content.
func (m Model) renderMode(b *strings.Builder) {
	b.WriteString("\n")
	switch m.mode {
	case modeCreatePlaylist:
		// The trailing block is a simple text cursor for the inline input.
		fmt.Fprintf(b, "%s %s\n", titleStyle.Render("New playlist:"), m.nameInput+"▌")
	case modeConfirmDeletePlaylist:
		fmt.Fprintf(b, "Delete playlist %q? (y/n)\n", m.confirmTarget)
	case modeConfirmRemoveTrack:
		name := ""
		if m.trackCursor < len(m.playlistTracks) {
			name = m.playlistTracks[m.trackCursor].Name
		}
		fmt.Fprintf(b, "Remove %q from %q? (y/n)\n", name, m.openPlaylist)
	case modeAddToPlaylist:
		m.renderPicker(b)
	}
}

// renderPicker draws the target-playlist list for adding the selected song.
func (m Model) renderPicker(b *strings.Builder) {
	song := ""
	if m.addSongIndex < len(m.songs) {
		song = m.songs[m.addSongIndex].Name
	}
	fmt.Fprintf(b, "%s\n", titleStyle.Render("Add "+song+" to playlist:"))
	if !m.playlistsLoaded {
		b.WriteString("Loading playlists…\n")
		return
	}
	if len(m.playlists) == 0 {
		b.WriteString("No playlists to add to.\n")
		return
	}
	start, end := listWindow(m.pickerCursor, len(m.playlists))
	for i := start; i < end; i++ {
		renderRow(b, m.playlists[i], i == m.pickerCursor)
	}
}

func (m Model) renderTabBar() string {
	parts := make([]string, 0, tabCount)
	for i := range int(tabCount) {
		t := tab(i)
		if t == m.tab {
			parts = append(parts, activeTabStyle.Render(t.title()))
		} else {
			parts = append(parts, inactiveTabStyle.Render(t.title()))
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) renderNowPlayingTab(b *strings.Builder) {
	if m.artwork != "" {
		b.WriteString(m.artwork + "\n")
	}

	switch {
	case m.err != nil:
		b.WriteString(errStyle.Render("error: "+m.err.Error()) + "\n\n")
	case m.track == nil:
		b.WriteString("Nothing playing.\n\n")
	default:
		fmt.Fprintf(b, "%s — %s\n%s\n[%s]\n\n", m.track.Name, m.track.Artist, m.track.Album, m.track.State)
	}

	fmt.Fprintf(b, "shuffle: %s  repeat: %s  volume: %d\n", onOff(m.shuffle), m.repeat, m.volume)
}

func (m Model) renderPlaylistsTab(b *strings.Builder) {
	if m.listErr != nil {
		b.WriteString(errStyle.Render("error: "+m.listErr.Error()) + "\n")
		return
	}
	if m.openPlaylist != "" {
		fmt.Fprintf(b, "%s\n", titleStyle.Render(m.openPlaylist))
		if m.playlistTracks == nil {
			b.WriteString("Loading tracks…\n")
			return
		}
		renderTrackList(b, m.playlistTracks, m.trackCursor, "No tracks in this playlist.")
		return
	}
	if !m.playlistsLoaded {
		b.WriteString("Loading playlists…\n")
		return
	}
	if len(m.playlists) == 0 {
		b.WriteString("No playlists found.\n")
		return
	}
	start, end := listWindow(m.playlistCursor, len(m.playlists))
	for i := start; i < end; i++ {
		renderRow(b, m.playlists[i], i == m.playlistCursor)
	}
}

func (m Model) renderSongsTab(b *strings.Builder) {
	if m.listErr != nil {
		b.WriteString(errStyle.Render("error: "+m.listErr.Error()) + "\n")
		return
	}
	if !m.songsLoaded {
		b.WriteString("Loading songs…\n")
		return
	}
	renderTrackList(b, m.songs, m.songCursor, "No songs found.")
}

// renderTrackList renders a scrollable list of library tracks with the cursor
// row highlighted, or emptyMsg when there are none.
func renderTrackList(b *strings.Builder, tracks []musicapp.LibraryTrack, cursor int, emptyMsg string) {
	if len(tracks) == 0 {
		b.WriteString(emptyMsg + "\n")
		return
	}
	start, end := listWindow(cursor, len(tracks))
	for i := start; i < end; i++ {
		renderRow(b, tracks[i].Name+" — "+tracks[i].Artist, i == cursor)
	}
}

// renderRow writes a single list row, prefixing the selected row with a cursor
// marker and highlighting it.
func renderRow(b *strings.Builder, text string, selected bool) {
	if selected {
		b.WriteString(cursorStyle.Render("> "+text) + "\n")
		return
	}
	b.WriteString("  " + text + "\n")
}

func (m Model) helpLine() string {
	switch m.mode {
	case modeCreatePlaylist:
		return "type a name  enter: create  esc: cancel"
	case modeConfirmDeletePlaylist, modeConfirmRemoveTrack:
		return "y: confirm  n/esc: cancel"
	case modeAddToPlaylist:
		return "↑↓: move  enter: add  esc: cancel"
	}
	switch m.tab {
	case tabPlaylists:
		if m.openPlaylist != "" {
			return "↑↓: move  enter: play  d: remove  esc: back  tab: switch tab  q: quit"
		}
		return "↑↓: move  enter: open  a: new  d: delete  r: reload  tab: switch tab  q: quit"
	case tabSongs:
		return "↑↓: move  enter: play  a: add to playlist  r: reload  tab: switch tab  q: quit"
	default:
		return "space: play/pause  n: next  p: prev  s: shuffle  c: repeat  +/-: volume  tab: switch tab  q: quit"
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// An open overlay captures every key (so typed text isn't read as a global
	// shortcut, and q doesn't quit mid-prompt) until it's confirmed or canceled.
	if m.mode != modeNone {
		return m.handleModeKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab":
		return m.switchTab(1)
	case "shift+tab":
		return m.switchTab(-1)
	}

	// Transport controls work from any tab so playback can be driven while
	// browsing. They own no arrow/enter/esc keys, so they never collide with
	// list navigation.
	if cmd, handled := m.transportCmd(msg); handled {
		return m, cmd
	}

	switch m.tab {
	case tabPlaylists:
		return m.handlePlaylistsKey(msg)
	case tabSongs:
		return m.handleSongsKey(msg)
	default:
		if msg.String() == "r" {
			return m, fetchAll(m.client)
		}
	}
	return m, nil
}

// transportCmd handles the playback-control keys shared by every tab. handled
// is false for any key it doesn't own, so the caller can fall through to
// tab-specific navigation.
func (m Model) transportCmd(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case " ":
		if m.track != nil && m.track.State == "playing" {
			return doAction(m.client.Pause), true
		}
		return doAction(m.client.Play), true
	case "n":
		return doAction(m.client.Next), true
	case "p":
		return doAction(m.client.Previous), true
	case "s":
		enable := !m.shuffle
		return doAction(func(ctx context.Context) error {
			return m.client.SetShuffle(ctx, enable)
		}), true
	case "c":
		next := nextRepeatMode(m.repeat)
		return doAction(func(ctx context.Context) error {
			return m.client.SetRepeat(ctx, next)
		}), true
	case "+", "=":
		level := clamp(m.volume+5, 0, 100)
		return doAction(func(ctx context.Context) error {
			return m.client.SetVolume(ctx, level)
		}), true
	case "-", "_":
		level := clamp(m.volume-5, 0, 100)
		return doAction(func(ctx context.Context) error {
			return m.client.SetVolume(ctx, level)
		}), true
	}
	return nil, false
}

// switchTab cycles the active tab by dir (+1/-1) and lazily loads that tab's
// library data the first time it's shown.
func (m Model) switchTab(dir int) (tea.Model, tea.Cmd) {
	m.tab = tab((int(m.tab) + dir + int(tabCount)) % int(tabCount))
	m.listErr = nil
	return m, m.ensureTabLoaded()
}

// ensureTabLoaded returns a fetch command for the active tab when its data
// hasn't been loaded yet, or nil when there's nothing to fetch.
func (m Model) ensureTabLoaded() tea.Cmd {
	switch m.tab {
	case tabPlaylists:
		if !m.playlistsLoaded {
			return fetchPlaylists(m.client)
		}
	case tabSongs:
		if !m.songsLoaded {
			return fetchSongs(m.client)
		}
	}
	return nil
}

// handleModeKey dispatches a key to the handler for the active overlay.
func (m Model) handleModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeCreatePlaylist:
		return m.handleCreatePlaylistKey(msg)
	case modeConfirmDeletePlaylist:
		return m.handleConfirmDeleteKey(msg)
	case modeConfirmRemoveTrack:
		return m.handleConfirmRemoveTrackKey(msg)
	case modeAddToPlaylist:
		return m.handleAddToPlaylistKey(msg)
	}
	return m, nil
}

func (m Model) handleCreatePlaylistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.mode = modeNone
		m.nameInput = ""
	case tea.KeyEnter:
		name := strings.TrimSpace(m.nameInput)
		m.mode = modeNone
		m.nameInput = ""
		if name == "" {
			return m, nil
		}
		return m, createPlaylist(m.client, name)
	case tea.KeyBackspace:
		m.nameInput = trimLastRune(m.nameInput)
	case tea.KeySpace:
		m.nameInput += " "
	case tea.KeyRunes:
		m.nameInput += string(msg.Runes)
	}
	return m, nil
}

func (m Model) handleConfirmDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.confirmTarget
		m.mode = modeNone
		m.confirmTarget = ""
		return m, deletePlaylist(m.client, name)
	case "n", "N", "esc":
		m.mode = modeNone
		m.confirmTarget = ""
	}
	return m, nil
}

func (m Model) handleConfirmRemoveTrackKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		playlist, index := m.openPlaylist, m.trackCursor
		m.mode = modeNone
		return m, removePlaylistTrack(m.client, playlist, index)
	case "n", "N", "esc":
		m.mode = modeNone
	}
	return m, nil
}

func (m Model) handleAddToPlaylistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.pickerCursor = moveCursor(m.pickerCursor, -1, len(m.playlists))
	case "down", "j":
		m.pickerCursor = moveCursor(m.pickerCursor, 1, len(m.playlists))
	case "enter":
		if len(m.playlists) == 0 {
			return m, nil
		}
		index, target := m.addSongIndex, m.playlists[m.pickerCursor]
		m.mode = modeNone
		return m, addSongToPlaylist(m.client, index, target)
	case "esc":
		m.mode = modeNone
	}
	return m, nil
}

func (m Model) handlePlaylistsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Drilled into a playlist: navigate and play its tracks.
	if m.openPlaylist != "" {
		switch msg.String() {
		case "up", "k":
			m.trackCursor = moveCursor(m.trackCursor, -1, len(m.playlistTracks))
		case "down", "j":
			m.trackCursor = moveCursor(m.trackCursor, 1, len(m.playlistTracks))
		case "enter":
			if len(m.playlistTracks) == 0 {
				return m, nil
			}
			name, index := m.openPlaylist, m.trackCursor
			return m, doAction(func(ctx context.Context) error {
				return m.client.PlayPlaylistTrack(ctx, name, index)
			})
		case "d":
			if len(m.playlistTracks) == 0 {
				return m, nil
			}
			m.mode = modeConfirmRemoveTrack
		case "esc", "backspace", "left", "h":
			m.openPlaylist = ""
			m.playlistTracks = nil
			m.trackCursor = 0
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		m.playlistCursor = moveCursor(m.playlistCursor, -1, len(m.playlists))
	case "down", "j":
		m.playlistCursor = moveCursor(m.playlistCursor, 1, len(m.playlists))
	case "enter", "right", "l":
		if len(m.playlists) == 0 {
			return m, nil
		}
		m.openPlaylist = m.playlists[m.playlistCursor]
		m.playlistTracks = nil
		m.trackCursor = 0
		return m, fetchPlaylistTracks(m.client, m.openPlaylist)
	case "a":
		m.mode = modeCreatePlaylist
		m.nameInput = ""
	case "d":
		if len(m.playlists) == 0 {
			return m, nil
		}
		m.mode = modeConfirmDeletePlaylist
		m.confirmTarget = m.playlists[m.playlistCursor]
	case "r":
		m.playlistsLoaded = false
		return m, fetchPlaylists(m.client)
	}
	return m, nil
}

func (m Model) handleSongsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.songCursor = moveCursor(m.songCursor, -1, len(m.songs))
	case "down", "j":
		m.songCursor = moveCursor(m.songCursor, 1, len(m.songs))
	case "enter":
		if len(m.songs) == 0 {
			return m, nil
		}
		index := m.songCursor
		return m, doAction(func(ctx context.Context) error {
			return m.client.PlaySong(ctx, index)
		})
	case "a":
		if len(m.songs) == 0 {
			return m, nil
		}
		m.mode = modeAddToPlaylist
		m.addSongIndex = m.songCursor
		m.pickerCursor = 0
		// The picker needs the playlist list; load it lazily if the Playlists
		// tab hasn't been visited yet.
		if !m.playlistsLoaded {
			return m, fetchPlaylists(m.client)
		}
	case "r":
		m.songsLoaded = false
		return m, fetchSongs(m.client)
	}
	return m, nil
}

// trackIdentity distinguishes tracks for the purpose of deciding whether to
// refetch artwork; State changes (playing/paused) don't count as a new track.
func trackIdentity(t *musicapp.Track) string {
	if t == nil {
		return ""
	}
	return t.Name + "\x00" + t.Artist + "\x00" + t.Album
}

// trimLastRune drops the final rune of s, backing over a full multi-byte
// character so backspace in the name prompt deletes one glyph, not one byte.
func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
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

// maxCursor is the highest valid cursor index for a list of n items (0 for an
// empty list, so the cursor never goes negative).
func maxCursor(n int) int {
	if n <= 0 {
		return 0
	}
	return n - 1
}

// moveCursor shifts cur by delta within [0, n-1], clamping at the ends rather
// than wrapping.
func moveCursor(cur, delta, n int) int {
	return clamp(cur+delta, 0, maxCursor(n))
}

// listWindow returns the [start, end) slice bounds of the visible window of a
// list of length n, scrolled so cursor stays on screen.
func listWindow(cursor, n int) (start, end int) {
	if n <= listVisibleRows {
		return 0, n
	}
	start = cursor - listVisibleRows/2
	start = clamp(start, 0, n-listVisibleRows)
	return start, start + listVisibleRows
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

func fetchArtwork(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()

		data, err := c.Artwork(ctx)
		if err != nil {
			if errors.Is(err, musicapp.ErrNoArtwork) || errors.Is(err, musicapp.ErrNothingPlaying) {
				return artworkMsg{}
			}
			return artworkMsg{err: err}
		}

		render := renderArtwork
		if kittyGraphicsAvailable() {
			render = renderArtworkKitty
		}
		rendered, err := render(data)
		if err != nil {
			return artworkMsg{err: err}
		}
		return artworkMsg{rendered: rendered}
	}
}

func fetchPlaylists(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		playlists, err := c.Playlists(ctx)
		return playlistsMsg{playlists: playlists, err: err}
	}
}

func fetchPlaylistTracks(c *musicapp.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		tracks, err := c.PlaylistTracks(ctx, name)
		return playlistTracksMsg{playlist: name, tracks: tracks, err: err}
	}
}

// createPlaylist and deletePlaylist mutate the playlist set, then reload it so
// the change shows immediately; on success they return a playlistsMsg (reusing
// the list-reload path), on failure a libraryActionMsg carrying the error.
func createPlaylist(c *musicapp.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		if err := c.CreatePlaylist(ctx, name); err != nil {
			return libraryActionMsg{err: err}
		}
		playlists, err := c.Playlists(ctx)
		return playlistsMsg{playlists: playlists, err: err}
	}
}

func deletePlaylist(c *musicapp.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		if err := c.DeletePlaylist(ctx, name); err != nil {
			return libraryActionMsg{err: err}
		}
		playlists, err := c.Playlists(ctx)
		return playlistsMsg{playlists: playlists, err: err}
	}
}

// removePlaylistTrack removes a track from a playlist, then reloads that
// playlist's tracks so the removal shows immediately.
func removePlaylistTrack(c *musicapp.Client, playlist string, index int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		if err := c.RemovePlaylistTrack(ctx, playlist, index); err != nil {
			return libraryActionMsg{err: err}
		}
		tracks, err := c.PlaylistTracks(ctx, playlist)
		return playlistTracksMsg{playlist: playlist, tracks: tracks, err: err}
	}
}

// addSongToPlaylist copies a library song into a playlist. Nothing visible in
// the Songs tab changes, so it only reports success/failure via libraryActionMsg.
func addSongToPlaylist(c *musicapp.Client, index int, playlist string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return libraryActionMsg{err: c.AddSongToPlaylist(ctx, index, playlist)}
	}
}

func fetchSongs(c *musicapp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()
		songs, err := c.Songs(ctx, songsPageSize, 0)
		return songsMsg{songs: songs, err: err}
	}
}

// fetchAll refreshes track info and shuffle/repeat/volume status in
// parallel; each fetch is an independent osascript invocation.
func fetchAll(c *musicapp.Client) tea.Cmd {
	return tea.Batch(fetchNowPlaying(c), fetchShuffle(c), fetchRepeat(c), fetchVolume(c))
}

// tick schedules a tickMsg after refreshInterval to drive periodic polling.
func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
