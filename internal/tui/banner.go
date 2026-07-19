package tui

import "github.com/charmbracelet/lipgloss"

// wordmark is figlet's "block" font rendering of "amuse", generated with
// `figlet -f block amuse` and pasted in verbatim.
const wordmark = `  _|_|_|  _|_|_|  _|_|    _|    _|    _|_|_|    _|_|
_|    _|  _|    _|    _|  _|    _|  _|_|      _|_|_|_|
_|    _|  _|    _|    _|  _|    _|      _|_|  _|
  _|_|_|  _|    _|    _|    _|_|_|  _|_|_|      _|_|_|  `

var (
	wordmarkStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	taglineStyle  = lipgloss.NewStyle().Faint(true)
)

// banner renders the startup wordmark shown once before the TUI takes over.
func banner() string {
	return wordmarkStyle.Render(wordmark) + "\n" + taglineStyle.Render("♪ Apple Music, from your terminal.")
}
