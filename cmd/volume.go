package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var volumeCmd = &cobra.Command{
	Use:   "volume [0-100]",
	Short: "Get or set the playback volume",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		if len(args) == 0 {
			level, err := client.Volume(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), level)
			return nil
		}

		level, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("volume must be an integer 0-100: %w", err)
		}
		return client.SetVolume(cmd.Context(), level)
	},
}

func init() {
	rootCmd.AddCommand(volumeCmd)
}
