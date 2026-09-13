//go:build release

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// The build script copies frontend/dist into backend/static right before
// running `go build -tags release`, and this embeds that directory's
// contents straight into the compiled binary. The result: a single .exe
// with no external files required to serve the UI.
//
//go:embed all:static
var embeddedFrontend embed.FS

// frontendHandler (release build) serves the React app from inside the
// binary itself instead of off disk.
func frontendHandler() http.Handler {
	sub, err := fs.Sub(embeddedFrontend, "static")
	if err != nil {
		log.Fatalf("embedded frontend missing — did the build script run `npm run build` and copy frontend/dist to backend/static first? %v", err)
	}
	return http.FileServer(http.FS(sub))
}

// dbPath (release build) puts the database in a "database" folder next to
// wherever the .exe itself lives, resolved from the executable's own path
// rather than the current working directory — so double-clicking the
// .exe from Explorer, a desktop shortcut, or a different folder always
// finds (or creates) the same database, instead of it changing depending
// on where the process happened to be launched from.
func dbPath() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("could not determine executable path: %v", err)
	}
	return filepath.Join(filepath.Dir(exe), "database", "snippets.json")
}

// configPath (release build) — the config file sits right next to the
// .exe itself, same reasoning as dbPath: resolved from the executable's
// own location so it's found regardless of the working directory the
// process happened to be launched from.
func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("could not determine executable path: %v", err)
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}
