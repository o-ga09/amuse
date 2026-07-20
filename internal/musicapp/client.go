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
