package cmd

import "github.com/spf13/cobra"

var playCmd = &cobra.Command{
	Use:   "play",
	Short: "Start playback",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newClient().Play(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(playCmd)
}
