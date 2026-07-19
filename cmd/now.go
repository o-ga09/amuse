package cmd

import (
	"errors"
	"fmt"

	"github.com/o-ga09/amuse/internal/musicapp"
	"github.com/spf13/cobra"
)

var nowCmd = &cobra.Command{
	Use:   "now",
	Short: "Show the currently playing track",
	RunE: func(cmd *cobra.Command, _ []string) error {
		track, err := newClient().NowPlaying(cmd.Context())
		if err != nil {
			if errors.Is(err, musicapp.ErrNothingPlaying) {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing playing.")
				return nil
			}
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\nAlbum: %s\nState: %s\n", track.Name, track.Artist, track.Album, track.State)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(nowCmd)
}
