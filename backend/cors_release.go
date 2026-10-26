//go:build release

package main

import "net/http"

// withCORS (release build) is intentionally a no-op. In production the
// UI is embedded in the binary and served from the same origin as the
// API, so no cross-origin access is needed — and adding none means
// browsers refuse to let any other origin read API responses.
//
// The dev-server allowlist lives in cors_dev.go and is not compiled into
// release binaries.
func withCORS(next http.Handler) http.Handler {
	return next
}

// devTrustedOrigins (release build) is empty: no development origins are
// trusted by the origin check in a shipped binary.
func devTrustedOrigins() []string { return nil }
