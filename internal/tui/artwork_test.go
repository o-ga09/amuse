package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// testPNG returns a tiny solid-color PNG, standing in for real album artwork.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestRenderArtwork_ProducesOneLinePerRow(t *testing.T) {
	got, err := renderArtwork(testPNG(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != artworkRows {
		t.Fatalf("got %d lines, want %d", len(lines), artworkRows)
	}
	for _, line := range lines {
		if !strings.Contains(line, "\x1b[38;2;") || !strings.Contains(line, "\x1b[48;2;") {
			t.Errorf("line %q missing truecolor fg/bg escapes", line)
		}
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Errorf("line %q does not reset styling at the end", line)
		}
	}
}

func TestRenderArtwork_AveragesSolidColor(t *testing.T) {
	got, err := renderArtwork(testPNG(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The test image is a single solid color, so every sub-cell in every
	// cell is equal (all "foreground"), rendering as a full block glyph in
	// that exact color, regardless of grid size.
	want := "\x1b[38;2;200;100;50m\x1b[48;2;200;100;50m█"
	if !strings.Contains(got, want) {
		t.Errorf("renderArtwork() = %q, want to contain %q", got, want)
	}
}

func TestQuantizeSextant(t *testing.T) {
	white := rgbColor{255, 255, 255}
	black := rgbColor{0, 0, 0}
	tests := []struct {
		name     string
		colors   [sextantSubcells]rgbColor
		wantMask uint8
		wantFg   rgbColor
		wantBg   rgbColor
	}{
		{
			name:     "all equal collapses to a full block",
			colors:   [sextantSubcells]rgbColor{{10, 10, 10}, {10, 10, 10}, {10, 10, 10}, {10, 10, 10}, {10, 10, 10}, {10, 10, 10}},
			wantMask: 0b111111,
			wantFg:   rgbColor{10, 10, 10},
			wantBg:   rgbColor{10, 10, 10},
		},
		{
			name:     "bright top four, dark bottom two",
			colors:   [sextantSubcells]rgbColor{white, white, white, white, black, black},
			wantMask: 0b001111,
			wantFg:   white,
			wantBg:   black,
		},
		{
			name:     "single bright top-left sub-cell",
			colors:   [sextantSubcells]rgbColor{white, black, black, black, black, black},
			wantMask: 0b000001,
			wantFg:   white,
			wantBg:   black,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg, bg, mask := quantizeSextant(tt.colors)
			if mask != tt.wantMask {
				t.Errorf("mask = %06b, want %06b", mask, tt.wantMask)
			}
			if fg != tt.wantFg {
				t.Errorf("fg = %+v, want %+v", fg, tt.wantFg)
			}
			if bg != tt.wantBg {
				t.Errorf("bg = %+v, want %+v", bg, tt.wantBg)
			}
		})
	}
}

func TestSextantGlyph(t *testing.T) {
	tests := []struct {
		name string
		mask uint8
		want rune
	}{
		{"empty is a space", 0b000000, ' '},
		{"full is a full block", 0b111111, '█'},
		{"left column is a left half block", 0b010101, '▌'},
		{"right column is a right half block", 0b101010, '▐'},
		{"first sextant codepoint", 0b000001, '\U0001FB00'},
		{"last sextant codepoint", 0b111110, '\U0001FB3B'},
		{"mask just past the left-column gap", 0b010110, '\U0001FB14'},
		{"mask just past the right-column gap", 0b101011, '\U0001FB28'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sextantGlyph(tt.mask); got != tt.want {
				t.Errorf("sextantGlyph(%06b) = %q (U+%04X), want %q (U+%04X)", tt.mask, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestRenderArtwork_RejectsUndecodableData(t *testing.T) {
	if _, err := renderArtwork([]byte("not an image")); err == nil {
		t.Fatal("expected an error for undecodable data")
	}
}
