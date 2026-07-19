package cmd

import (
	"fmt"
	"os"

	"github.com/o-ga09/amuse/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     version.Name,
	Short:   "amuse controls Apple Music (Music.app) from the terminal",
	Version: version.Version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
