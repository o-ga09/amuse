package cmd

import (
	"errors"
	"fmt"

	"github.com/o-ga09/amuse/internal/musicapp"
	"github.com/spf13/cobra"
)

var repeatCmd = &cobra.Command{
	Use:   "repeat [off|one|all]",
	Short: "Get or set repeat mode",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		if len(args) == 0 {
			mode, err := client.Repeat(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), mode)
			return nil
		}

		mode := musicapp.RepeatMode(args[0])
		if err := client.SetRepeat(cmd.Context(), mode); err != nil {
			if errors.Is(err, musicapp.ErrInvalidRepeatMode) {
				return fmt.Errorf("repeat must be one of off|one|all, got %q", args[0])
			}
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(repeatCmd)
}
