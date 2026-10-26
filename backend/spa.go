package main

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// newSPAHandler serves a single-page app from fsys.
//
// The React app uses BrowserRouter, so URLs like /snippets/3 or /settings
// don't exist as files — they're client-side routes. A plain
// http.FileServer answers those with 404 on refresh or bookmark. This
// handler:
//
//   - serves real files (JS/CSS/images/index.html) as-is;
//   - serves index.html for any other extension-less GET/HEAD path, so the
//     router can take over;
//   - returns a real 404 for missing files that look like assets
//     (/assets/old-hash.js, /favicon.ico) and for unknown /api/ paths, so
//     a stale asset URL doesn't get HTML back and fail with a confusing
//     "unexpected token <" error;
//   - never lists directories (/assets/ falls through to index.html).
func newSPAHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))

	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		// Clone so we don't mutate the caller's request, then let
		// FileServer serve "/" (= index.html) with its usual
		// Last-Modified / conditional-request handling.
		r2 := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/"
		r2.URL = &u
		// The shell references hashed asset names that change every
		// build, so it must always be revalidated.
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r2)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			files.ServeHTTP(w, r)
			return
		}

		clean := path.Clean("/" + r.URL.Path)
		if clean == "/" {
			serveIndex(w, r)
			return
		}
		// API paths are handled by their own routes; anything unknown
		// there is a genuine 404, not an app screen.
		if clean == "/api" || strings.HasPrefix(clean, "/api/") {
			http.NotFound(w, r)
			return
		}

		if info, err := fs.Stat(fsys, strings.TrimPrefix(clean, "/")); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		// Missing file or a directory. Asset-looking paths 404;
		// route-looking paths get the app shell.
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r)
	})
}
