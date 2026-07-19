package musicapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNothingPlaying is returned by NowPlaying when Music.app's player state is stopped.
var ErrNothingPlaying = errors.New("nothing playing")

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

func (c *Client) Next(ctx context.Context) error {
	_, err := c.runner.Run(ctx, `tell application "Music" to next track`)
	return err
}

func (c *Client) Previous(ctx context.Context) error {
	_, err := c.runner.Run(ctx, `tell application "Music" to previous track`)
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
