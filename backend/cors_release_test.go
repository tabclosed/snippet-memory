//go:build release

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The dev-server allowlist must not exist in release binaries.
func TestReleaseHasNoCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := withCORS(ok)

	for _, origin := range []string{"http://localhost:5173", "http://127.0.0.1:5173", "https://evil.example"} {
		for _, method := range []string{"GET", "OPTIONS"} {
			r := httptest.NewRequest(method, "/api/snippets", nil)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			for k := range w.Header() {
				if len(k) > 14 && k[:14] == "Access-Control" {
					t.Errorf("%s from %s got %s", method, origin, k)
				}
			}
		}
	}
}
