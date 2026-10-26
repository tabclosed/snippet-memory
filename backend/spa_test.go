package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":        {Data: []byte("<html>SHELL</html>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
		"favicon.svg":       {Data: []byte("<svg/>")},
	}
	h := newSPAHandler(fsys)

	cases := []struct {
		method, path string
		code         int
		body         string // substring; "" = don't check
	}{
		{"GET", "/", 200, "SHELL"},
		{"GET", "/snippets/1", 200, "SHELL"},
		{"GET", "/snippets/1/edit", 200, "SHELL"},
		{"GET", "/settings", 200, "SHELL"},
		{"HEAD", "/settings", 200, ""},
		{"GET", "/assets/app-abc.js", 200, "console.log"},
		{"GET", "/favicon.svg", 200, "<svg/>"},
		{"GET", "/assets/old-hash.js", 404, ""}, // stale asset: real 404, not HTML
		{"GET", "/favicon.ico", 404, ""},
		{"GET", "/assets/", 200, "SHELL"}, // no directory listing
		{"GET", "/api/nope", 404, ""},
		{"GET", "/api", 404, ""},
		{"GET", "/../../etc/passwd", 200, "SHELL"}, // cleaned, never escapes the FS
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != c.code {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, w.Code, c.code)
		}
		if c.body != "" && !strings.Contains(w.Body.String(), c.body) {
			t.Errorf("%s %s body = %q, want it to contain %q", c.method, c.path, w.Body.String(), c.body)
		}
	}

	// Shell must be revalidated so new deploys are picked up.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/settings", nil))
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	// Non-GET requests are not rewritten to the shell.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/snippets/1", nil))
	if strings.Contains(w.Body.String(), "SHELL") {
		t.Errorf("POST was served the app shell")
	}
}
