package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"strings"
)

// artworkCols and artworkRows size the thumbnail in terminal cells. Each cell
// packs a 2x3 sub-grid of samples (via a sextant block glyph), so the sampled
// pixel grid is twice as wide and three times as tall as this; see
// renderArtwork.
const (
	artworkCols = 24
	artworkRows = 12
)

// sextantSubcells is how many samples one cell holds: 2 wide x 3 tall.
const sextantSubcells = 6

// sextantGlyph maps a 6-bit "which sub-cells are foreground-colored" mask to
// the Unicode block sextant glyph showing exactly that pattern. Bit i marks
// sub-cell i+1, numbered top-to-bottom, left-to-right:
//
//	1 2
//	3 4
//	5 6
//
// The block sextants live at U+1FB00..U+1FB3B in increasing mask order, but
// the four masks that already had glyphs elsewhere are omitted from that
// range, so those are special-cased and the rest are offset past the gaps.
func sextantGlyph(mask uint8) rune {
	switch mask {
	case 0b000000:
		return ' '
	case 0b010101: // left column (sub-cells 1,3,5)
		return '▌'
	case 0b101010: // right column (sub-cells 2,4,6)
		return '▐'
	case 0b111111:
		return '█'
	}
	offset := int(mask) - 1 // U+1FB00 is mask 1; mask 0 has no codepoint here
	if mask > 0b010101 {
		offset--
	}
	if mask > 0b101010 {
		offset--
	}
	// The four special masks are handled above, so offset here lands in
	// [0,59], keeping the sum well inside the U+1FB00..U+1FB3B block.
	return rune(0x1FB00 + offset) // #nosec G115
}

// rgbColor is a plain 24-bit color, used instead of image/color to keep the
// sextant quantization math (averages, luminance) in ordinary ints.
type rgbColor struct {
	r, g, b uint8
}

// renderArtwork approximates raw album artwork bytes (JPEG/PNG/etc., as
// stored by Music.app) using 24-bit-color sextant block glyphs: each terminal
// cell packs a 2x3 sub-grid of samples into one glyph shape plus a
// foreground/background color pair, roughly sextupling perceived resolution
// per cell over a plain half-block (which only splits top/bottom). This is
// the same technique tools like chafa's "symbols" mode use, and works in any
// terminal that supports ANSI truecolor escapes (effectively all of them) by
// drawing into the ordinary text grid, so it composes safely with
// bubbletea's redraw cycle.
//
// Native terminal image protocols (Kitty, iTerm2) were tried and reverted:
// both draw to a layer outside the text grid bubbletea doesn't know how to
// account for when it recomputes redraw height, which repeatedly produced
// stale/overlapping frames in real terminals that couldn't be fully chased
// down without live access to those terminals to iterate against.
func renderArtwork(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode artwork: %w", err)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "", fmt.Errorf("decoded artwork has zero dimensions")
	}

	var b strings.Builder
	for row := range artworkRows {
		for col := range artworkCols {
			x0 := bounds.Min.X + (col*2)*w/(artworkCols*2)
			xm := bounds.Min.X + (col*2+1)*w/(artworkCols*2)
			x1 := bounds.Min.X + (col*2+2)*w/(artworkCols*2)
			y0 := bounds.Min.Y + (row*3)*h/(artworkRows*3)
			ya := bounds.Min.Y + (row*3+1)*h/(artworkRows*3)
			yb := bounds.Min.Y + (row*3+2)*h/(artworkRows*3)
			y1 := bounds.Min.Y + (row*3+3)*h/(artworkRows*3)

			// Sub-cells in mask-bit order (see sextantGlyph): 1..6 top to
			// bottom, left to right.
			cells := [sextantSubcells]rgbColor{
				boxColor(img, x0, y0, xm, ya), // 1 top-left
				boxColor(img, xm, y0, x1, ya), // 2 top-right
				boxColor(img, x0, ya, xm, yb), // 3 mid-left
				boxColor(img, xm, ya, x1, yb), // 4 mid-right
				boxColor(img, x0, yb, xm, y1), // 5 bottom-left
				boxColor(img, xm, yb, x1, y1), // 6 bottom-right
			}

			fg, bg, mask := quantizeSextant(cells)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%c", fg.r, fg.g, fg.b, bg.r, bg.g, bg.b, sextantGlyph(mask))
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String(), nil
}

// quantizeSextant reduces the 6 sampled sub-cell colors to a 2-color
// (foreground/background) approximation plus the sextant mask that reproduces
// their spatial pattern: each sub-cell is classified into the brighter or
// darker half by comparing its luminance to the average of all six, then each
// group is averaged down to a single color. A perfectly flat cell (all 6
// equal) ends up as a single foreground color filling the whole glyph.
func quantizeSextant(colors [sextantSubcells]rgbColor) (fg, bg rgbColor, mask uint8) {
	lum := func(c rgbColor) int { return 299*int(c.r) + 587*int(c.g) + 114*int(c.b) }

	sum := 0
	for _, c := range colors {
		sum += lum(c)
	}
	avg := sum / len(colors)

	var fgSum, bgSum [3]int
	var fgN, bgN int
	for i, c := range colors {
		if lum(c) >= avg {
			fgSum[0] += int(c.r)
			fgSum[1] += int(c.g)
			fgSum[2] += int(c.b)
			fgN++
			mask |= 1 << uint(i) // colors[i] is sub-cell i+1, i.e. bit i
		} else {
			bgSum[0] += int(c.r)
			bgSum[1] += int(c.g)
			bgSum[2] += int(c.b)
			bgN++
		}
	}

	// Each summed component is a sum of values already in [0,0xff]; dividing
	// by the (positive) count that produced it keeps the result in
	// [0,0xff], so narrowing to uint8 can't overflow.
	fg = rgbColor{uint8(fgSum[0] / fgN), uint8(fgSum[1] / fgN), uint8(fgSum[2] / fgN)} // #nosec G115
	if bgN == 0 {
		bg = fg
	} else {
		bg = rgbColor{uint8(bgSum[0] / bgN), uint8(bgSum[1] / bgN), uint8(bgSum[2] / bgN)} // #nosec G115
	}
	return fg, bg, mask
}

// boxColor averages the pixels in [x0,x1) x [y0,y1) into a single RGB color,
// smoothing out sampling noise that nearest-neighbor picking would leave in
// a grid this coarse. If artwork is smaller than the sample grid
// (upsampling), the row/col math in renderArtwork can produce an empty
// range; widen it to at least a single pixel so there's always something to
// average.
func boxColor(img image.Image, x0, y0, x1, y1 int) rgbColor {
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}

	var sumR, sumG, sumB, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			sumR += uint64(cr >> 8)
			sumG += uint64(cg >> 8)
			sumB += uint64(cb >> 8)
			n++
		}
	}
	// RGBA channels are pre-multiplied into [0,0xffff]; averaging values
	// already narrowed to [0,0xff] and dividing by a positive n keeps the
	// result within [0,0xff], so this narrowing can't overflow.
	return rgbColor{uint8(sumR / n), uint8(sumG / n), uint8(sumB / n)} // #nosec G115
}
