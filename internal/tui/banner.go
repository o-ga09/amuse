package tui

import "github.com/charmbracelet/lipgloss"

var bannerStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("13")).
	Padding(0, 2)

// banner renders the startup box shown once before the TUI takes over.
func banner() string {
	return bannerStyle.Render("♪ amuse\n\nApple Music, from your terminal.")
}
