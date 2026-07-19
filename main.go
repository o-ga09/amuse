package main

import (
	"log/slog"
	"os"

	"github.com/o-ga09/amuse/cmd"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	cmd.Execute()
}
