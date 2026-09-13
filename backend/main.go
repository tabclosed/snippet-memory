package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	store, err := NewStore(dbPath())
	if err != nil {
		log.Fatalf("failed to load store: %v", err)
	}

	cfg, err := LoadConfig(configPath())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// auth.json lives right next to snippets.json.
	authPath := filepath.Join(filepath.Dir(dbPath()), "auth.json")
	passwords, err := NewPasswordStore(authPath)
	if err != nil {
		log.Fatalf("failed to load password store: %v", err)
	}
	sessions := NewSessionStore()
	limiter := newLoginLimiter()

	a := &api{store: store}
	auth := &authAPI{passwords: passwords, sessions: sessions, limiter: limiter}

	mux := http.NewServeMux()

	// Auth endpoints. login/logout are public by nature; check requires
	// an existing session (that's the whole point — it's how the
	// frontend confirms one exists on page load).
	mux.HandleFunc("POST /api/login", auth.login)
	mux.HandleFunc("POST /api/logout", auth.logout)
	mux.HandleFunc("GET /api/auth/check", auth.requireAuth(auth.check))
	mux.HandleFunc("POST /api/settings/password", auth.requireAuth(auth.changePassword))

	// Snippets REST API — all behind the login. The "METHOD /path"
	// pattern syntax (Go 1.22+) lets the mux route by HTTP method
	// directly, no extra router library needed.
	mux.HandleFunc("GET /api/snippets", auth.requireAuth(a.listSnippets))
	mux.HandleFunc("POST /api/snippets", auth.requireAuth(a.createSnippet))
	mux.HandleFunc("GET /api/snippets/{id}", auth.requireAuth(a.getSnippet))
	mux.HandleFunc("PUT /api/snippets/{id}", auth.requireAuth(a.updateSnippet))
	mux.HandleFunc("PATCH /api/snippets/{id}/order", auth.requireAuth(a.updateSnippetOrder))
	mux.HandleFunc("DELETE /api/snippets/{id}", auth.requireAuth(a.deleteSnippet))

	// Serve the built React app — including its own login screen, so
	// this is intentionally NOT behind requireAuth (a logged-out visitor
	// still needs to receive the page that shows them the login form).
	// In a normal `go run .`/`go build` this reads frontend/dist off
	// disk (see frontend_dev.go); in a release build
	// (`go build -tags release`) it's embedded straight into the binary
	// instead (see frontend_release.go), so a shipped .exe is a single
	// self-contained file.
	mux.Handle("/", frontendHandler())

	handler := withCORS(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)

	// TLS is opt-in: set both env vars to point at a cert/key pair and
	// the server switches to HTTPS. Left unset (the default), it's
	// plain HTTP — fine for localhost, but if this is reachable from
	// other devices on your network, either set these or put a reverse
	// proxy (Caddy, nginx, Tailscale, etc.) in front that terminates
	// TLS itself; otherwise the password and session cookie travel in
	// cleartext. See the "Password protection" section in the README.
	certFile := os.Getenv("SNIPPET_MEMORY_TLS_CERT")
	keyFile := os.Getenv("SNIPPET_MEMORY_TLS_KEY")

	if certFile != "" && keyFile != "" {
		log.Printf("server listening on https://localhost%s", addr)
		err = http.ListenAndServeTLS(addr, certFile, keyFile, handler)
	} else {
		log.Printf("server listening on http://localhost%s", addr)
		err = http.ListenAndServe(addr, handler)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// allowedDevOrigins are the only origins the API will answer cross-origin
// requests for — specific known local dev-server addresses, not "*".
// In production the frontend and API share an origin, so browsers never
// even consult these headers there; this only matters if something
// talks to the Go server (:8080) directly from a different origin during
// development, bypassing Vite's same-origin proxy (see vite.config.ts).
var allowedDevOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://127.0.0.1:5173": true,
}

// withCORS lets the allow-listed dev origins above call the API
// cross-origin, with credentials (the session cookie) included, and
// leaves the CORS headers off entirely for anything else — so a
// browser talking to this server from an unrecognized origin can't
// read the response at all, rather than being allowed in by a blanket
// "Access-Control-Allow-Origin: *".
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedDevOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		// Tells caches/proxies the response varies by Origin, so a
		// response for one origin never gets served to another.
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
