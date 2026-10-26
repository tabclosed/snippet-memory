//go:build !release

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDevCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := withCORS(ok)

	do := func(method, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/snippets", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	w := do("GET", "http://localhost:5173")
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("dev origin not allowed: %v", w.Header())
	}
	if w := do("OPTIONS", "http://localhost:5173"); w.Code != 204 {
		t.Errorf("allowed preflight = %d, want 204", w.Code)
	}
	for _, evil := range []string{"https://evil.example", "http://localhost:8081", "null"} {
		w := do("GET", evil)
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("origin %q was allowed", evil)
		}
	}
	// Preflight from a disallowed origin must not get CORS approval.
	w = do("OPTIONS", "https://evil.example")
	if w.Code == 204 || w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Errorf("disallowed preflight approved: %d %v", w.Code, w.Header())
	}
}
