package cmd

import (
	"fmt"
	"os"

	"github.com/o-ga09/amuse/internal/tui"
	"github.com/o-ga09/amuse/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     version.Name,
	Short:   "amuse controls Apple Music (Music.app) from the terminal",
	Version: version.Version,
	// Runtime errors (e.g. the TUI failing to start) aren't usage mistakes;
	// don't dump the full help text for them, and print the error exactly
	// once via our own Execute() below instead of letting cobra do it too.
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return tui.Run(newClient())
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
