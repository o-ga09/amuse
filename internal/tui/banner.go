package tui

import "github.com/charmbracelet/lipgloss"

// wordmark is toilet's "mono12" font rendering of "amuse" (solid Unicode
// block glyphs, unlike figlet's thin "_"/"|" fonts), generated with
// `toilet -f mono12 amuse` and pasted in verbatim.
const wordmark = `  ▄█████▄  ████▄██▄  ██    ██  ▄▄█████▄   ▄████▄
  ▀ ▄▄▄██  ██ ██ ██  ██    ██  ██▄▄▄▄ ▀  ██▄▄▄▄██
 ▄██▀▀▀██  ██ ██ ██  ██    ██   ▀▀▀▀██▄  ██▀▀▀▀▀▀
 ██▄▄▄███  ██ ██ ██  ██▄▄▄███  █▄▄▄▄▄██  ▀██▄▄▄▄█
  ▀▀▀▀ ▀▀  ▀▀ ▀▀ ▀▀   ▀▀▀▀ ▀▀   ▀▀▀▀▀▀     ▀▀▀▀▀`

var (
	wordmarkStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	taglineStyle  = lipgloss.NewStyle().Faint(true)
)

// banner renders the startup wordmark shown once before the TUI takes over.
func banner() string {
	return wordmarkStyle.Render(wordmark) + "\n" + taglineStyle.Render("♪ Apple Music, from your terminal.")
}
