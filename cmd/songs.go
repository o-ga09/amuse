package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	songsLimit  int
	songsOffset int
)

var songsCmd = &cobra.Command{
	Use:   "songs",
	Short: "List local library tracks",
	Long: "List local library tracks as tab-separated name, artist, and album — " +
		"one per line, for piping into other tools. Use --limit and --offset to " +
		"page through a large library.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if songsOffset < 0 {
			return fmt.Errorf("--offset must be >= 0, got %d", songsOffset)
		}
		tracks, err := newClient().Songs(cmd.Context(), songsLimit, songsOffset)
		if err != nil {
			return err
		}
		printLibraryTracks(cmd, tracks)
		return nil
	},
}

func init() {
	songsCmd.Flags().IntVar(&songsLimit, "limit", 50, "maximum number of tracks to list (0 for all)")
	songsCmd.Flags().IntVar(&songsOffset, "offset", 0, "number of tracks to skip from the start")
	rootCmd.AddCommand(songsCmd)
}
