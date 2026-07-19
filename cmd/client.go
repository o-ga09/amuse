package cmd

import "github.com/o-ga09/amuse/internal/musicapp"

func newClient() *musicapp.Client {
	return musicapp.NewClient(musicapp.OSARunner{})
}
