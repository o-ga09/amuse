package cmd

import "github.com/spf13/cobra"

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newClient().Next(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(nextCmd)
}
