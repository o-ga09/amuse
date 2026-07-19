package musicapp

import (
	"context"
	"errors"
	"fmt"
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
