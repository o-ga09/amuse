package musicapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ErrNothingPlaying is returned by NowPlaying when Music.app's player state is stopped.
var ErrNothingPlaying = errors.New("nothing playing")

// ErrNoArtwork is returned by Artwork when the current track has no artwork.
var ErrNoArtwork = errors.New("no artwork")

// Track describes the currently playing (or paused) track.
type Track struct {
	Name   string
	Artist string
	Album  string
	State  string // "playing" or "paused"
}

// Client controls Music.app via a Runner.
type Client struct {
	runner Runner
}

func NewClient(r Runner) *Client {
	return &Client{runner: r}
}

func (c *Client) Play(ctx context.Context) error {
	_, err := c.runner.Run(ctx, `tell application "Music" to play`)
	return err
}

func (c *Client) Pause(ctx context.Context) error {
	_, err := c.runner.Run(ctx, `tell application "Music" to pause`)
	return err
}

// nextScript and previousScript guard against the queue-boundary dead-end: once
// playback reaches the end of the queue Music.app's player state goes to
// stopped, and "next track"/"previous track" become no-ops with no current
// track to move from, leaving the TUI stuck on "Nothing playing." (issue #22).
// Falling back to "play" restarts the current playlist instead of no-op'ing.
const nextScript = `tell application "Music"
	if player state is stopped then
		play
	else
		next track
	end if
end tell`

const previousScript = `tell application "Music"
	if player state is stopped then
		play
	else
		previous track
	end if
end tell`

func (c *Client) Next(ctx context.Context) error {
	_, err := c.runner.Run(ctx, nextScript)
	return err
}

func (c *Client) Previous(ctx context.Context) error {
	_, err := c.runner.Run(ctx, previousScript)
	return err
}

// AppleScript string literals don't support "\n" escapes, so the field
// separator is built from (ASCII character 10) to get a real line feed that
// strings.Split can match on below.
const nowPlayingScript = `tell application "Music"
	if player state is stopped then
		return "stopped"
	end if
	set nl to (ASCII character 10)
	set trackName to name of current track
	set trackArtist to artist of current track
	set trackAlbum to album of current track
	set trackState to player state as string
	return trackName & nl & trackArtist & nl & trackAlbum & nl & trackState
end tell`

func (c *Client) NowPlaying(ctx context.Context) (*Track, error) {
	out, err := c.runner.Run(ctx, nowPlayingScript)
	if err != nil {
		return nil, err
	}
	if out == "stopped" {
		return nil, ErrNothingPlaying
	}

	fields := strings.Split(out, "\n")
	if len(fields) != 4 {
		return nil, fmt.Errorf("unexpected osascript output: %q", out)
	}

	return &Track{
		Name:   fields[0],
		Artist: fields[1],
		Album:  fields[2],
		State:  fields[3],
	}, nil
}

// artworkScript writes the current track's artwork, in its original file
// format (JPEG/PNG/etc.), to the given path. osascript's stdout mangles raw
// binary data, so the script writes it to a file itself and returns a status
// word instead; the caller reads the bytes back off disk.
const artworkScript = `tell application "Music"
	if player state is stopped then
		return "stopped"
	end if
	if (count of artworks of current track) is 0 then
		return "none"
	end if
	set artData to raw data of artwork 1 of current track
	set fileRef to open for access (POSIX file "%s") with write permission
	set eof fileRef to 0
	write artData to fileRef
	close access fileRef
	return "ok"
end tell`

// Artwork returns the current track's artwork as raw, undecoded image bytes
// (JPEG, PNG, etc., whatever Music.app stored them as).
func (c *Client) Artwork(ctx context.Context) ([]byte, error) {
	tmp, err := os.CreateTemp("", "amuse-artwork-*.bin")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("close temp file: %w", closeErr)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	// tmpPath comes from os.CreateTemp, never from external input, so
	// interpolating it into the script can't inject AppleScript syntax.
	out, err := c.runner.Run(ctx, fmt.Sprintf(artworkScript, tmpPath))
	if err != nil {
		return nil, err
	}

	switch out {
	case "stopped":
		return nil, ErrNothingPlaying
	case "none":
		return nil, ErrNoArtwork
	case "ok":
		// tmpPath is the os.CreateTemp path from above, never external input.
		data, readErr := os.ReadFile(tmpPath) // #nosec G304
		if readErr != nil {
			return nil, fmt.Errorf("read artwork file: %w", readErr)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unexpected osascript output: %q", out)
	}
}

// LibraryTrack is a track from the local library, as returned by Search and Songs.
type LibraryTrack struct {
	Name   string
	Artist string
	Album  string
}

// escapeAppleScriptString neutralizes a free-text value so it can be
// interpolated into an AppleScript double-quoted string literal without
// injecting syntax. Backslashes and double quotes are escaped so the value
// can't break out of the literal; other control characters are dropped because
// AppleScript literals can't hold a raw newline/tab (they'd be a syntax error)
// and they have no meaning in a library search term. See
// .claude/rules/security-review.md.
func escapeAppleScriptString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r < 0x20 || r == 0x7f:
			// drop control characters
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Track fields are joined with a tab and rows with a newline (ASCII 9 / 10),
// mirroring nowPlayingScript's line-feed convention. Track names containing a
// literal tab or newline are rare enough to accept the parsing ambiguity.
const searchScript = `tell application "Music"
	set nl to (ASCII character 10)
	set tb to (ASCII character 9)
	set out to ""
	set matches to (search library playlist 1 for "%s")
	repeat with t in matches
		set out to out & (name of t) & tb & (artist of t) & tb & (album of t) & nl
	end repeat
	return out
end tell`

// Search returns local-library tracks matching query. It searches the library
// only (AppleScript's `search` command), never the full Apple Music catalog.
func (c *Client) Search(ctx context.Context, query string) ([]LibraryTrack, error) {
	script := fmt.Sprintf(searchScript, escapeAppleScriptString(query))
	out, err := c.runner.Run(ctx, script)
	if err != nil {
		return nil, err
	}
	return parseLibraryTracks(out), nil
}

// songsScript lists library tracks from a 1-based offset. limit and offset are
// interpolated as %d (digits only, no injection risk); a limit <= 0 lists every
// track from the offset onward.
//
// Crucially it never materializes `every track` of the library: on a large
// library that pulls tens of thousands of track references across the Apple
// event boundary and blows past the caller's timeout (osascript gets SIGKILLed,
// surfacing as "signal: killed"). Instead it bounds the work to the requested
// window and reads each property straight off the range specifier
// (`name of (tracks m thru n of pl)`), which Music.app answers as one bulk Apple
// event per property — orders of magnitude faster than iterating track-by-track,
// and, unlike reading properties off an intermediate resolved list, it works for
// iCloud/Apple Music "shared track"s too.
//
// A range that resolves to a single track is special-cased: AppleScript collapses
// `name of (tracks n thru n ...)` to a scalar instead of a one-element list, so
// the window of one is read by index instead.
const songsScript = `tell application "Music"
	set nl to (ASCII character 10)
	set tb to (ASCII character 9)
	set pl to library playlist 1
	set total to (count of tracks of pl)
	set startIdx to %d + 1
	set lim to %d
	if lim <= 0 then
		set endIdx to total
	else
		set endIdx to startIdx + lim - 1
	end if
	if endIdx > total then set endIdx to total
	if startIdx > total then return ""
	if startIdx is endIdx then
		set t to track startIdx of pl
		return (name of t) & tb & (artist of t) & tb & (album of t) & nl
	end if
	set theNames to name of (tracks startIdx thru endIdx of pl)
	set theArtists to artist of (tracks startIdx thru endIdx of pl)
	set theAlbums to album of (tracks startIdx thru endIdx of pl)
	set out to ""
	repeat with i from 1 to (count of theNames)
		set out to out & (item i of theNames) & tb & (item i of theArtists) & tb & (item i of theAlbums) & nl
	end repeat
	return out
end tell`

// ErrInvalidPagination is returned by Songs for a negative limit or offset.
var ErrInvalidPagination = errors.New("invalid pagination")

// Songs lists local-library tracks starting at offset (0-based). A limit <= 0
// lists every remaining track.
func (c *Client) Songs(ctx context.Context, limit, offset int) ([]LibraryTrack, error) {
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset %d must be >= 0", ErrInvalidPagination, offset)
	}
	script := fmt.Sprintf(songsScript, offset, limit)
	out, err := c.runner.Run(ctx, script)
	if err != nil {
		return nil, err
	}
	return parseLibraryTracks(out), nil
}

// parseLibraryTracks decodes the tab/newline-delimited output of the search and
// songs scripts. Rows without exactly three fields are skipped.
func parseLibraryTracks(out string) []LibraryTrack {
	if out == "" {
		return nil
	}
	var tracks []LibraryTrack
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		tracks = append(tracks, LibraryTrack{
			Name:   fields[0],
			Artist: fields[1],
			Album:  fields[2],
		})
	}
	return tracks
}

// playlistsScript builds a newline-separated list of playlist names manually
// rather than relying on AppleScript's default comma-space list formatting,
// which is ambiguous for names that themselves contain a comma.
const playlistsScript = `tell application "Music"
	set nl to (ASCII character 10)
	set out to ""
	repeat with p in playlists
		set out to out & (name of p) & nl
	end repeat
	return out
end tell`

// Playlists lists the names of every playlist in the local library.
func (c *Client) Playlists(ctx context.Context) ([]string, error) {
	out, err := c.runner.Run(ctx, playlistsScript)
	if err != nil {
		return nil, err
	}
	return parsePlaylistNames(out), nil
}

// parsePlaylistNames splits the newline-delimited output of playlistsScript,
// skipping blank lines (trailing newline, empty names).
func parsePlaylistNames(out string) []string {
	if out == "" {
		return nil
	}
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}
		names = append(names, line)
	}
	return names
}

// playlistTracksScript lists the tracks of the first playlist whose name
// matches. The name is free-text and must be escaped before interpolation; see
// escapeAppleScriptString.
// playlistTracksScript reads each property straight off the track-range
// specifier for the same bulk-Apple-event speed and shared-track compatibility
// as songsScript (see its comment), with the same single-track special case.
const playlistTracksScript = `tell application "Music"
	set nl to (ASCII character 10)
	set tb to (ASCII character 9)
	set pl to (first playlist whose name is "%s")
	set total to (count of tracks of pl)
	if total is 0 then return ""
	if total is 1 then
		set t to track 1 of pl
		return (name of t) & tb & (artist of t) & tb & (album of t) & nl
	end if
	set theNames to name of (tracks 1 thru total of pl)
	set theArtists to artist of (tracks 1 thru total of pl)
	set theAlbums to album of (tracks 1 thru total of pl)
	set out to ""
	repeat with i from 1 to (count of theNames)
		set out to out & (item i of theNames) & tb & (item i of theArtists) & tb & (item i of theAlbums) & nl
	end repeat
	return out
end tell`

// PlaylistTracks lists the tracks of the named playlist, in playlist order. The
// index of each track matches the trackIndex expected by PlayPlaylistTrack.
func (c *Client) PlaylistTracks(ctx context.Context, name string) ([]LibraryTrack, error) {
	script := fmt.Sprintf(playlistTracksScript, escapeAppleScriptString(name))
	out, err := c.runner.Run(ctx, script)
	if err != nil {
		return nil, err
	}
	return parseLibraryTracks(out), nil
}

// PlayPlaylist starts playback of the named playlist from its first track.
func (c *Client) PlayPlaylist(ctx context.Context, name string) error {
	script := fmt.Sprintf(`tell application "Music" to play (first playlist whose name is "%s")`,
		escapeAppleScriptString(name))
	_, err := c.runner.Run(ctx, script)
	return err
}

// playPlaylistTrackScript starts the named playlist at the track addressed by
// %d (a 1-based playlist position) and continues in playlist order.
//
// Playing a bare track object (`play track N of playlist X`) always leaves
// Music's *current playlist* set to the library, so playback wouldn't advance
// through the playlist — it would fall into library order instead (issue: the
// TUI didn't advance to the next playlist track). The only way to get
// playlist-context continuation is to `play` the whole playlist, then walk to
// the target track. Music has no "start playlist at track N" primitive and
// current playlist/current track are read-only, so walking is unavoidable.
//
// The walk is deliberate: `next track`/`previous track` need time to settle, so
// each step waits for `index of current track` to actually change before firing
// the next (a self-paced pace that's both faster and more reliable than a fixed
// delay). It's bidirectional because `play pl` resumes at the playlist's last
// position, not necessarily track 1, and stops early if a step can't change the
// index (playlist boundary). Shuffle is forced off — index-order navigation is
// meaningless with it on. Volume is muted around the walk so the tracks skipped
// past aren't briefly audible, and restored even on error.
const playPlaylistTrackScript = `tell application "Music"
	set pl to (first playlist whose name is "%s")
	set target to %d
	set shuffle enabled to false
	set savedVol to sound volume
	set sound volume to 0
	try
		play pl
		repeat 30 times
			if player state is playing then exit repeat
			delay 0.1
		end repeat
		repeat until (index of current track) is target
			set prev to (index of current track)
			if prev < target then
				next track
			else
				previous track
			end if
			set changed to false
			repeat 25 times
				if (index of current track) is not prev then
					set changed to true
					exit repeat
				end if
				delay 0.1
			end repeat
			if not changed then exit repeat
		end repeat
		set sound volume to savedVol
	on error errMsg number errNum
		set sound volume to savedVol
		error errMsg number errNum
	end try
end tell`

// PlayPlaylistTrack starts the named playlist at the track addressed by its
// 0-based position in the order PlaylistTracks returns, and continues in
// playlist order from there. The name is escaped; the index is an int, so %d
// yields digits only and can't inject syntax. See playPlaylistTrackScript for
// why this walks the playlist rather than playing the track directly, and note
// it can take a while for tracks deep in a playlist — call it with a generous
// context timeout.
func (c *Client) PlayPlaylistTrack(ctx context.Context, name string, index int) error {
	if index < 0 {
		return fmt.Errorf("%w: index %d must be >= 0", ErrInvalidPagination, index)
	}
	script := fmt.Sprintf(playPlaylistTrackScript, escapeAppleScriptString(name), index+1)
	_, err := c.runner.Run(ctx, script)
	return err
}

// PlaySong plays the track at the given 0-based position in the main library
// (library playlist 1), matching the ordering of Songs with offset 0.
func (c *Client) PlaySong(ctx context.Context, index int) error {
	if index < 0 {
		return fmt.Errorf("%w: index %d must be >= 0", ErrInvalidPagination, index)
	}
	// index is an int, so %d can't produce anything but digits - no injection risk.
	script := fmt.Sprintf(`tell application "Music" to play (track %d of library playlist 1)`, index+1)
	_, err := c.runner.Run(ctx, script)
	return err
}

// ErrEmptyPlaylistName is returned by CreatePlaylist for a blank name.
var ErrEmptyPlaylistName = errors.New("empty playlist name")

// CreatePlaylist creates a new, empty playlist with the given name. The name is
// free-text and is escaped before interpolation; see escapeAppleScriptString.
// Music.app allows duplicate playlist names, so this always adds a new one.
func (c *Client) CreatePlaylist(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyPlaylistName
	}
	script := fmt.Sprintf(`tell application "Music" to make new playlist with properties {name:"%s"}`,
		escapeAppleScriptString(name))
	_, err := c.runner.Run(ctx, script)
	return err
}

// DeletePlaylist deletes the first playlist whose name matches. The name is
// escaped; see escapeAppleScriptString.
func (c *Client) DeletePlaylist(ctx context.Context, name string) error {
	script := fmt.Sprintf(`tell application "Music" to delete (first playlist whose name is "%s")`,
		escapeAppleScriptString(name))
	_, err := c.runner.Run(ctx, script)
	return err
}

// AddSongToPlaylist copies the library track at the given 0-based position (the
// same ordering as Songs/PlaySong) into the named playlist. The playlist name is
// escaped; the index is an int, so %d yields digits only and can't inject syntax.
func (c *Client) AddSongToPlaylist(ctx context.Context, index int, playlist string) error {
	if index < 0 {
		return fmt.Errorf("%w: index %d must be >= 0", ErrInvalidPagination, index)
	}
	script := fmt.Sprintf(
		`tell application "Music" to duplicate (track %d of library playlist 1) to (first playlist whose name is "%s")`,
		index+1, escapeAppleScriptString(playlist))
	_, err := c.runner.Run(ctx, script)
	return err
}

// RemovePlaylistTrack deletes the track at the given 0-based position (the same
// ordering PlaylistTracks returns) from the named playlist. Deleting a track
// from a user playlist only removes it from that playlist, not the library. The
// name is escaped; the index is an int, so %d can't inject syntax.
func (c *Client) RemovePlaylistTrack(ctx context.Context, playlist string, index int) error {
	if index < 0 {
		return fmt.Errorf("%w: index %d must be >= 0", ErrInvalidPagination, index)
	}
	script := fmt.Sprintf(
		`tell application "Music" to delete (track %d of (first playlist whose name is "%s"))`,
		index+1, escapeAppleScriptString(playlist))
	_, err := c.runner.Run(ctx, script)
	return err
}

// Shuffle reports whether shuffle is currently enabled.
func (c *Client) Shuffle(ctx context.Context) (bool, error) {
	out, err := c.runner.Run(ctx, `tell application "Music" to shuffle enabled`)
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

func (c *Client) SetShuffle(ctx context.Context, enabled bool) error {
	script := `tell application "Music" to set shuffle enabled to false`
	if enabled {
		script = `tell application "Music" to set shuffle enabled to true`
	}
	_, err := c.runner.Run(ctx, script)
	return err
}

// RepeatMode is one of the fixed set of values Music.app's "song repeat"
// property accepts.
type RepeatMode string

const (
	RepeatOff RepeatMode = "off"
	RepeatOne RepeatMode = "one"
	RepeatAll RepeatMode = "all"
)

func (m RepeatMode) valid() bool {
	switch m {
	case RepeatOff, RepeatOne, RepeatAll:
		return true
	}
	return false
}

// ErrInvalidRepeatMode is returned by SetRepeat for any mode outside off/one/all.
var ErrInvalidRepeatMode = errors.New("invalid repeat mode")

func (c *Client) Repeat(ctx context.Context) (RepeatMode, error) {
	out, err := c.runner.Run(ctx, `tell application "Music" to song repeat`)
	if err != nil {
		return "", err
	}
	return RepeatMode(out), nil
}

func (c *Client) SetRepeat(ctx context.Context, mode RepeatMode) error {
	if !mode.valid() {
		return fmt.Errorf("%w: %q", ErrInvalidRepeatMode, mode)
	}
	// mode is validated against the closed off/one/all set above, so
	// interpolating it here can't inject AppleScript syntax.
	script := fmt.Sprintf(`tell application "Music" to set song repeat to %s`, mode)
	_, err := c.runner.Run(ctx, script)
	return err
}

// ErrInvalidVolume is returned by SetVolume for levels outside 0-100.
var ErrInvalidVolume = errors.New("invalid volume")

func (c *Client) Volume(ctx context.Context) (int, error) {
	out, err := c.runner.Run(ctx, `tell application "Music" to sound volume`)
	if err != nil {
		return 0, err
	}
	level, convErr := strconv.Atoi(out)
	if convErr != nil {
		return 0, fmt.Errorf("unexpected osascript output: %q", out)
	}
	return level, nil
}

func (c *Client) SetVolume(ctx context.Context, level int) error {
	if level < 0 || level > 100 {
		return fmt.Errorf("%w: %d (must be 0-100)", ErrInvalidVolume, level)
	}
	// level is an int, so %d can't produce anything but digits - no injection risk.
	script := fmt.Sprintf(`tell application "Music" to set sound volume to %d`, level)
	_, err := c.runner.Run(ctx, script)
	return err
}
