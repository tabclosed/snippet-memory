package main

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func startServer(t *testing.T, srv *http.Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().String()
}

// waitClosed reports whether the server closes the connection within d.
func waitClosed(c net.Conn, d time.Duration) bool {
	c.SetReadDeadline(time.Now().Add(d))
	_, err := bufio.NewReader(c).ReadByte()
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return false // still open when our deadline hit
	}
	return err != nil // EOF / reset = server hung up
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
}

func TestSlowlorisHeaderIsCut(t *testing.T) {
	srv := newServer("", okHandler())
	srv.ReadHeaderTimeout = 200 * time.Millisecond // shrink for the test
	addr := startServer(t, srv)

	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nX-Slow: a")) // never finishes the headers
	if !waitClosed(c, 2*time.Second) {
		t.Fatal("server kept a half-sent request open past ReadHeaderTimeout")
	}
}

func TestControlNoTimeoutsStaysOpen(t *testing.T) {
	// Documents the original behaviour: a zero-value Server never hangs up.
	addr := startServer(t, &http.Server{Handler: okHandler()})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nX-Slow: a"))
	if waitClosed(c, 700*time.Millisecond) {
		t.Fatal("control unexpectedly closed")
	}
}

func TestIdleKeepAliveIsClosed(t *testing.T) {
	srv := newServer("", okHandler())
	srv.IdleTimeout = 200 * time.Millisecond
	addr := startServer(t, srv)

	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := br.ReadByte(); err == nil {
		t.Fatal("unexpected data")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("idle keep-alive connection was not closed")
	}
}

func TestNormalRequestStillWorks(t *testing.T) {
	ts := httptest.NewUnstartedServer(okHandler())
	s := newServer("", okHandler())
	ts.Config = s
	ts.Start()
	defer ts.Close()
	resp, err := http.Get(ts.URL)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("normal request failed: %v", err)
	}
	resp.Body.Close()
}

// ReadTimeout covers the whole body, so it must leave room for a
// max-size snippet over a slow-but-usable link (~100 KB/s).
func TestReadTimeoutAllowsMaxBodyOnSlowLink(t *testing.T) {
	const slowLink = 100 * 1024 // bytes/sec
	need := time.Duration(maxSnippetBodyBytes/slowLink) * time.Second
	if readTimeout < need {
		t.Fatalf("readTimeout %v too short for %d bytes at %d B/s (needs %v)", readTimeout, maxSnippetBodyBytes, slowLink, need)
	}
}
