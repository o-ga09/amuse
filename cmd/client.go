package cmd

import "github.com/o-ga09/amuse/internal/musicapp"

// newClient builds the musicapp client the commands use. It's a package-level
// variable rather than a plain function so tests can swap in a client backed by
// a fake musicapp.Runner instead of shelling out to real osascript.
var newClient = func() *musicapp.Client {
	return musicapp.NewClient(musicapp.OSARunner{})
}
