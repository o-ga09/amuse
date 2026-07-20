package cmd

import (
	"fmt"
	"strings"

	"github.com/o-ga09/amuse/internal/musicapp"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the local library and print matching tracks",
	Long: "Search the local library (Music.app's own search, not the full Apple " +
		"Music catalog) and print matching tracks as tab-separated name, artist, " +
		"and album — one per line, for piping into other tools.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		tracks, err := newClient().Search(cmd.Context(), query)
		if err != nil {
			return err
		}
		printLibraryTracks(cmd, tracks)
		return nil
	},
}

// printLibraryTracks writes tracks to stdout as tab-separated name/artist/album
// rows, the plain, pipe-friendly format the search/songs commands share.
func printLibraryTracks(cmd *cobra.Command, tracks []musicapp.LibraryTrack) {
	out := cmd.OutOrStdout()
	for _, t := range tracks {
		fmt.Fprintf(out, "%s\t%s\t%s\n", t.Name, t.Artist, t.Album)
	}
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
