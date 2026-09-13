//go:build !release

package main

import "net/http"

// frontendHandler (dev build, the default) serves the React app straight
// off disk from frontend/dist, the same way `go run .`/`go build` always
// worked. Run `npm run build` in frontend/ at least once before starting
// the server, or this directory won't exist yet.
func frontendHandler() http.Handler {
	return http.FileServer(http.Dir("../frontend/dist"))
}

// dbPath (dev build) matches the project layout: backend/ and database/
// are siblings, and `go run .`/`go build` are invoked from inside
// backend/, so a relative path is enough.
func dbPath() string {
	return "../database/snippets.json"
}

// configPath (dev build) — the config file sits at the project root,
// alongside database/, matching how dbPath resolves things for dev.
func configPath() string {
	return "../config.json"
}
