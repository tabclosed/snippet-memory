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

	// Behind a reverse proxy every request arrives from the proxy's own
	// address, so without this all clients would share one login-attempt
	// bucket (three bad guesses from anyone would lock out the owner).
	// List the proxy's IP/CIDR here and its X-Forwarded-For header is
	// trusted — only from those addresses, never from anyone else.
	// Example: SNIPPET_MEMORY_TRUSTED_PROXIES="127.0.0.1,::1"
	trustedProxies, err := parseTrustedProxies(os.Getenv("SNIPPET_MEMORY_TRUSTED_PROXIES"))
	if err != nil {
		log.Fatalf("SNIPPET_MEMORY_TRUSTED_PROXIES: %v", err)
	}
	if len(trustedProxies) > 0 {
		log.Printf("trusting X-Forwarded-For from %d proxy address range(s)", len(trustedProxies))
	}

	auth := &authAPI{passwords: passwords, sessions: sessions, limiter: limiter, trustedProxies: trustedProxies}

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

	// (withCORS inside that chain is build-tag specific — cors_dev.go /
	// cors_release.go: dev builds allow the Vite dev server cross-origin;
	// release builds add no CORS headers at all.)
	// Security headers, the cross-origin (CSRF) check and the JSON-body
	// check wrap the router; see security.go. If you serve this behind a
	// reverse proxy under a different public origin and writes start
	// failing with "cross-origin request blocked", either forward the
	// original Host header from the proxy or list the public origin here:
	// SNIPPET_MEMORY_TRUSTED_ORIGINS="https://snippets.example.com"
	handler, err := buildHandler(mux, parseTrustedOrigins(os.Getenv("SNIPPET_MEMORY_TRUSTED_ORIGINS")))
	if err != nil {
		log.Fatalf("SNIPPET_MEMORY_TRUSTED_ORIGINS: %v", err)
	}

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

	// Explicit http.Server so we can set timeouts (see server.go).
	srv := newServer(addr, handler)

	if certFile != "" && keyFile != "" {
		log.Printf("server listening on https://localhost%s", addr)
		err = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		log.Printf("server listening on http://localhost%s", addr)
		err = srv.ListenAndServe()
	}
	if err != nil {
		log.Fatal(err)
	}
}
