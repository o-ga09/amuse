package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var shuffleCmd = &cobra.Command{
	Use:   "shuffle [on|off]",
	Short: "Get or set shuffle",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		if len(args) == 0 {
			enabled, err := client.Shuffle(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), onOff(enabled))
			return nil
		}

		switch args[0] {
		case "on":
			return client.SetShuffle(cmd.Context(), true)
		case "off":
			return client.SetShuffle(cmd.Context(), false)
		default:
			return fmt.Errorf("shuffle must be \"on\" or \"off\", got %q", args[0])
		}
	},
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func init() {
	rootCmd.AddCommand(shuffleCmd)
}
