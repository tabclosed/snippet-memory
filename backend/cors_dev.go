//go:build !release

package main

import "net/http"

// allowedDevOrigins are the only origins the API will answer cross-origin
// requests for in a dev build — the local Vite dev server, not "*".
// This exists only for development, when the frontend (:5173) talks to
// the Go server (:8080) directly from a different origin, bypassing
// Vite's same-origin proxy (see vite.config.ts).
//
// It is deliberately compiled out of release builds (see cors_release.go):
// in production the frontend and API share an origin, and cookies aren't
// port-scoped, so shipping a localhost:5173 allowlist would let any page
// served from that port on the user's machine read their snippets.
var allowedDevOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://127.0.0.1:5173": true,
}

// withCORS (dev build) lets the allow-listed dev origins above call the
// API cross-origin, with credentials (the session cookie) included, and
// leaves the CORS headers off entirely for anything else — so a browser
// talking to this server from an unrecognized origin can't read the
// response at all, rather than being allowed in by a blanket
// "Access-Control-Allow-Origin: *".
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Tells caches/proxies the response varies by Origin, so a
		// response for one origin never gets served to another.
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if allowedDevOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			// Answer the preflight ourselves — but only for allowed
			// origins. Everyone else's preflight falls through to the
			// mux, which has no OPTIONS routes, so it gets no CORS
			// headers and the browser blocks the real request.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// devTrustedOrigins (dev build) lets the origin check (see security.go)
// accept the same Vite dev-server origins the CORS allowlist does —
// otherwise `npm run dev` could never write to the API.
func devTrustedOrigins() []string {
	origins := make([]string, 0, len(allowedDevOrigins))
	for o := range allowedDevOrigins {
		origins = append(origins, o)
	}
	return origins
}
