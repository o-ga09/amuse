package cmd

import "github.com/spf13/cobra"

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause playback",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newClient().Pause(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}
