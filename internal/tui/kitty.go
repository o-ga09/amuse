package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
)

// The Kitty graphics protocol with Unicode placeholders draws real pixels into
// cells the TUI still owns: each placeholder cell displays one cell's slice of
// a transmitted image, so bubbletea keeps accounting for the artwork's height
// and its redraw cycle stays intact — unlike direct image placements, which
// draw outside the text grid and desynced bubbletea's height math (see the
// note in artwork.go about the reverted native attempt). Terminals without
// this protocol fall back to the sextant renderer.
const (
	// kittyImageID is the image id we transmit under and reference from each
	// placeholder cell's foreground color. Only one artwork shows at a time,
	// so a fixed id is fine: each new track overwrites it.
	kittyImageID = 1
	// kittyPlaceholder is Kitty's Unicode placeholder code point.
	kittyPlaceholder = '\U0010EEEE'
	// kittyChunk is the max base64 payload bytes per transmit escape (protocol
	// limit).
	kittyChunk = 4096
)

// kittyDiacritics maps a 0-based row/column index to the combining mark Kitty
// uses to encode it in a placeholder cell. These are the first entries of
// Kitty's rowcolumn-diacritics table — enough to address the artworkCols x
// artworkRows box.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
}

// kittyGraphicsAvailable reports whether the current terminal supports the
// Kitty graphics protocol with Unicode placeholders, inferred from the
// environment. Terminal multiplexers (tmux/screen) need a passthrough we don't
// implement, so we stay on the block-glyph fallback under them.
func kittyGraphicsAvailable() bool {
	if os.Getenv("TMUX") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") {
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "WezTerm":
		return true
	}
	return false
}

// renderArtworkKitty transmits artwork as a Kitty graphics image and returns a
// grid of Unicode placeholder cells that display it in an artworkCols x
// artworkRows box. The returned string embeds the (chunked) transmit escape
// followed by the placeholder grid; it is byte-stable for a given image, so
// bubbletea's line diff retransmits only when the track actually changes.
func renderArtworkKitty(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode artwork: %w", err)
	}

	// Normalize to PNG: Music.app hands us JPEG or PNG, but Kitty's f=100
	// transmit format is PNG, so re-encode whatever we decoded.
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return "", fmt.Errorf("encode artwork as png: %w", err)
	}

	var b strings.Builder
	b.WriteString(kittyTransmit(pngBuf.Bytes()))
	b.WriteString(kittyPlaceholderGrid())
	return b.String(), nil
}

// kittyTransmit emits the chunked transmit-and-place sequence: it uploads the
// PNG bytes under kittyImageID and creates a Unicode-placeholder virtual
// placement spanning artworkCols x artworkRows cells. q=2 suppresses the
// terminal's acknowledgements so they can't leak into the TUI.
func kittyTransmit(pngData []byte) string {
	payload := base64.StdEncoding.EncodeToString(pngData)

	var b strings.Builder
	first := true
	for len(payload) > 0 {
		n := min(kittyChunk, len(payload))
		piece := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}

		if first {
			fmt.Fprintf(&b, "\x1b_Ga=T,i=%d,f=100,U=1,c=%d,r=%d,q=2,m=%d;%s\x1b\\", kittyImageID, artworkCols, artworkRows, more, piece)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, piece)
		}
	}
	return b.String()
}

// kittyPlaceholderGrid emits artworkCols x artworkRows placeholder cells. Every
// cell's foreground color encodes kittyImageID (24-bit) and carries explicit
// row+column diacritics so placement can't drift.
func kittyPlaceholderGrid() string {
	var b strings.Builder
	for row := range artworkRows {
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", (kittyImageID>>16)&0xff, (kittyImageID>>8)&0xff, kittyImageID&0xff)
		for col := range artworkCols {
			b.WriteRune(kittyPlaceholder)
			b.WriteRune(kittyDiacritics[row])
			b.WriteRune(kittyDiacritics[col])
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String()
}
