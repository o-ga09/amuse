<p align="center">
  <img src="docs/banner.svg" alt="amuse — Apple Music, from your terminal." width="700">
</p>

`amuse` controls Apple Music (Music.app) from the terminal on macOS, via AppleScript.

## Status

Playback control (play/pause/next/prev), now-playing, shuffle/repeat/volume, and an interactive TUI
are implemented. Library browsing, playlists, and search are not — Music.app's AppleScript dictionary
only exposes the local library, and querying/streaming Apple Music's full catalog would need the
separate MusicKit API (not implemented here).

## Install

```sh
brew install o-ga09/tap/amuse
```

or

```sh
go install github.com/o-ga09/amuse@latest
```

## Usage

```sh
amuse            # launch the interactive TUI
amuse play
amuse pause
amuse next
amuse prev
amuse now        # show the currently playing track

amuse shuffle    # get shuffle state
amuse shuffle on
amuse repeat     # get repeat mode
amuse repeat off|one|all
amuse volume     # get volume
amuse volume 50  # set volume (0-100)
```

TUI keybindings: `space` play/pause, `n` next, `p` prev, `s` toggle shuffle, `c` cycle repeat,
`+`/`-` volume, `r` refresh, `q` quit.

## Development

```sh
make build
make test
make lint
```
