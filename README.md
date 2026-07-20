<p align="center">
  <img src="docs/banner.svg" alt="amuse — Apple Music, from your terminal." width="700">
</p>

`amuse` controls Apple Music (Music.app) from the terminal on macOS, via AppleScript.

## Status

Playback control (play/pause/next/prev), now-playing, shuffle/repeat/volume, and an interactive TUI
are implemented. Library browsing, playlists, and search are not — Music.app's AppleScript dictionary
only exposes the local library, and querying/streaming Apple Music's full catalog would need the
separate MusicKit API (not implemented here).

> [!IMPORTANT]
> macOS only. `amuse` controls Music.app via AppleScript (`osascript`), which doesn't exist on
> Linux/Windows.

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

amuse search <query>   # search the local library; prints name<TAB>artist<TAB>album per match
amuse songs            # list local library tracks (--limit 50 --offset 0; --limit 0 for all)

amuse shuffle    # get shuffle state
amuse shuffle on
amuse repeat     # get repeat mode
amuse repeat off|one|all
amuse volume     # get volume
amuse volume 50  # set volume (0-100)
```

TUI keybindings: `space` play/pause, `n` next, `p` prev, `s` toggle shuffle, `c` cycle repeat,
`+`/`-` volume, `r` refresh, `q` quit.

## Troubleshooting

### "This computer is not authorized" when fetching artwork

When the current track is an Apple Music / iTunes Match item and the computer hasn't been
authorized, Music.app raises an authorization dialog instead of returning album artwork:

> This computer is not authorized. To use Apple Music or iTunes Match on this computer, you need
> to authorize the computer.

(The dialog is shown in your system language.)

Authorize the computer in Music.app: **Account → Authorizations → Authorize This Computer…**,
then sign in with your Apple Account.

<p align="center">
  <img src="docs/authorize-computer.gif" alt="Music.app: Account → Authorizations → Authorize This Computer, then sign in with your Apple Account." width="700">
</p>

> [!NOTE]
> macOS allows a single Apple Account to authorize up to 5 computers. If you're at the limit,
> deauthorize an old computer (or use **Deauthorize All** from a computer signed in to the account)
> before authorizing this one.

## Development

```sh
make build
make test
make lint
```

### Benchmarks

Run the benchmarks (they're excluded from a normal `make test`, which uses `-run`/no `-bench`):

```sh
go test ./internal/... -run='^$' -bench=. -benchmem
```

The point of these is to check where a command's time actually goes. Measured on an Apple M4
(darwin/arm64), the answer is: not in the Go code.

| Benchmark                       | Time per op | What it measures                              |
| ------------------------------- | ----------- | --------------------------------------------- |
| `Client_NowPlaying`             | ~48 ns      | script build + output parse (fake runner)     |
| `Client_Play`                   | ~2 ns       | script build (fake runner)                    |
| `Client_SetVolume`              | ~52 ns      | script build + parse (fake runner)            |
| `Model_Update_NowPlaying`       | ~51 ns      | TUI message handling                          |
| `Model_View`                    | ~2 µs       | TUI render (string building)                  |
| `OSARunner_RoundTrip`           | **~29 ms**  | one real `osascript` process spawn            |

So a single `osascript` spawn costs roughly **600,000×** the Go-side parsing per command — the Go
code is effectively free, and total latency is dominated entirely by process spawn. Any future
performance work should target the spawn count (e.g. a long-lived AppleScript/JXA process), not the
Go paths. `OSARunner_RoundTrip` runs a trivial no-op script and is skipped in `-short` mode and when
`osascript` isn't on `PATH` (i.e. on non-macOS CI).
