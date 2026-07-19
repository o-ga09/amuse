package cmd

import "github.com/spf13/cobra"

var prevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Skip to the previous track",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newClient().Previous(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(prevCmd)
}
