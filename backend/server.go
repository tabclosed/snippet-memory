package main

import (
	"net/http"
	"time"
)

// Server timeouts. http.ListenAndServe uses a zero-value http.Server,
// which has NO timeouts at all: a client that opens a connection and
// dribbles out a header one byte at a time (slowloris), or just sits
// idle, holds a goroutine and a file descriptor forever.
//
// The values are generous for a personal tool on a home network, but
// finite. ReadTimeout covers the whole request including the body, so it
// has to leave room for the largest allowed payload (5 MB snippets, see
// maxSnippetBodyBytes) over a slow connection.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 64 << 10 // 64 KiB; the default is 1 MiB
)

// newServer builds the http.Server used for both HTTP and HTTPS.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}
