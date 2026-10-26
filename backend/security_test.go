package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- helpers --------------------------------------------------------------

func stubMux() *http.ServeMux {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("reached")) }
	mux.HandleFunc("/api/", ok)
	mux.HandleFunc("/", ok)
	return mux
}

func mustBuild(t *testing.T, mux http.Handler, trusted ...string) http.Handler {
	t.Helper()
	h, err := buildHandler(mux, trusted)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

type secReq struct {
	method, path, host, body string
	headers                  map[string]string
}

func (q secReq) do(h http.Handler) *httptest.ResponseRecorder {
	if q.host == "" {
		q.host = "app.local:8080"
	}
	if q.path == "" {
		q.path = "/api/snippets"
	}
	r := httptest.NewRequest(q.method, "http://"+q.host+q.path, strings.NewReader(q.body))
	for k, v := range q.headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func jsonH(extra map[string]string) map[string]string {
	m := map[string]string{"Content-Type": "application/json"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// ---- security headers -----------------------------------------------------

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	h := mustBuild(t, stubMux())
	want := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	// Success, API, an error produced by the middleware itself (403), and
	// a 415 — headers must be on all of them.
	cases := []secReq{
		{method: "GET", path: "/"},
		{method: "GET", path: "/api/snippets"},
		{method: "POST", path: "/api/snippets", body: "{}", headers: jsonH(map[string]string{"Sec-Fetch-Site": "cross-site"})},
		{method: "POST", path: "/api/snippets", body: "x", headers: map[string]string{"Content-Type": "text/plain"}},
	}
	for _, c := range cases {
		w := c.do(h)
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%s %s (%d): %s = %q, want %q", c.method, c.path, w.Code, k, got, v)
			}
		}
		if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("Permissions-Policy") == "" {
			t.Errorf("%s %s (%d): missing CSP/Permissions-Policy", c.method, c.path, w.Code)
		}
	}
}

func TestCSPDirectives(t *testing.T) {
	dirs := map[string]string{}
	for _, d := range strings.Split(contentSecurityPolicy, ";") {
		d = strings.TrimSpace(d)
		name, val, _ := strings.Cut(d, " ")
		dirs[name] = val
	}
	if dirs["script-src"] != "'self'" {
		t.Errorf("script-src = %q; must be exactly 'self' (no unsafe-inline / unsafe-eval)", dirs["script-src"])
	}
	for _, name := range []string{"default-src", "script-src", "object-src", "base-uri", "frame-ancestors"} {
		if strings.Contains(dirs[name], "unsafe") || strings.Contains(dirs[name], "*") {
			t.Errorf("%s = %q is too permissive", name, dirs[name])
		}
	}
	if dirs["frame-ancestors"] != "'none'" || dirs["object-src"] != "'none'" || dirs["base-uri"] != "'none'" {
		t.Errorf("frame-ancestors/object-src/base-uri must be 'none': %v", dirs)
	}
	if !strings.Contains(dirs["connect-src"], "https://api.github.com") {
		t.Errorf("connect-src must allow the Settings page's update check: %q", dirs["connect-src"])
	}
}

func TestCacheControlOnlyForAPI(t *testing.T) {
	h := mustBuild(t, stubMux())
	if got := (secReq{method: "GET", path: "/api/snippets"}).do(h).Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("API Cache-Control = %q, want no-store", got)
	}
	if got := (secReq{method: "GET", path: "/assets/app.js"}).do(h).Header().Get("Cache-Control"); got != "" {
		t.Errorf("static Cache-Control = %q, want unset", got)
	}
}

func TestHSTSOnlyOverHTTPS(t *testing.T) {
	h := mustBuild(t, stubMux())

	if got := (secReq{method: "GET", path: "/"}).do(h).Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS sent over plain HTTP: %q", got)
	}
	// Terminated by a proxy.
	w := secReq{method: "GET", path: "/", headers: map[string]string{"X-Forwarded-Proto": "https"}}.do(h)
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("no HSTS behind an https proxy")
	}
	// Direct TLS.
	r := httptest.NewRequest("GET", "https://app.local/", nil)
	r.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	got := rec.Header().Get("Strict-Transport-Security")
	if got == "" || strings.Contains(got, "includeSubDomains") || strings.Contains(got, "preload") {
		t.Errorf("HSTS over TLS = %q; want max-age only", got)
	}
}

// ---- origin check (CSRF) --------------------------------------------------

func TestCrossOriginWrites(t *testing.T) {
	h := mustBuild(t, stubMux())
	const host = "app.local:8080"

	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"same-origin browser", "POST", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://" + host}, 200},
		{"typed URL / bookmark (none)", "POST", map[string]string{"Sec-Fetch-Site": "none"}, 200},
		{"cross-site page", "POST", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, 403},
		{"same-site, other port (SameSite=Lax gap)", "POST", map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://app.local:9999"}, 403},
		{"cross-site PUT", "PUT", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"cross-site PATCH", "PATCH", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"cross-site DELETE", "DELETE", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"no fetch metadata, Origin matches Host", "POST", map[string]string{"Origin": "http://" + host}, 200},
		{"no fetch metadata, foreign Origin", "POST", map[string]string{"Origin": "https://evil.example"}, 403},
		{"non-browser client (curl): no headers at all", "POST", nil, 200},
		// Reads aren't state-changing; they are protected by the session
		// cookie + CORS, and must keep working for link navigation.
		{"cross-site GET is not blocked", "GET", map[string]string{"Sec-Fetch-Site": "cross-site"}, 200},
	}
	for _, c := range cases {
		w := secReq{method: c.method, host: host, body: "{}", headers: jsonH(c.headers)}.do(h)
		if w.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.want)
		}
	}
}

func TestBlockedResponseIsJSONError(t *testing.T) {
	h := mustBuild(t, stubMux())
	w := secReq{method: "POST", body: "{}", headers: jsonH(map[string]string{"Sec-Fetch-Site": "cross-site"})}.do(h)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"error"`) || strings.Contains(w.Body.String(), "reached") {
		t.Errorf("got %d %q", w.Code, w.Body.String())
	}
}

func TestTrustedOriginsFromConfig(t *testing.T) {
	h := mustBuild(t, stubMux(), "https://snippets.example.com")
	// Behind a proxy that doesn't forward the public Host: Host is the
	// internal address, Origin is the public one.
	req := func(origin string) int {
		return secReq{method: "POST", host: "127.0.0.1:8080", body: "{}", headers: jsonH(map[string]string{"Origin": origin})}.do(h).Code
	}
	if got := req("https://snippets.example.com"); got != 200 {
		t.Errorf("configured origin = %d, want 200", got)
	}
	if got := req("https://evil.example"); got != 403 {
		t.Errorf("other origin = %d, want 403", got)
	}
}

func TestInvalidTrustedOriginRejectedAtStartup(t *testing.T) {
	for _, bad := range []string{"snippets.example.com", "localhost:5173", "https://a.example/path", "*"} {
		if _, err := buildHandler(stubMux(), []string{bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestParseTrustedOrigins(t *testing.T) {
	got := parseTrustedOrigins(" https://a.example , ,https://b.example:8443,")
	if len(got) != 2 || got[0] != "https://a.example" || got[1] != "https://b.example:8443" {
		t.Errorf("got %v", got)
	}
	if len(parseTrustedOrigins("")) != 0 {
		t.Error("empty input should yield nothing")
	}
}

// ---- JSON content type ----------------------------------------------------

func TestRequireJSONBody(t *testing.T) {
	h := mustBuild(t, stubMux())
	cases := []struct {
		name, method, ctype, body string
		want                      int
	}{
		{"json", "POST", "application/json", `{"a":1}`, 200},
		{"json with charset", "POST", "application/json; charset=utf-8", `{"a":1}`, 200},
		{"text/plain smuggling", "POST", "text/plain", `{"a":1}`, 415},
		{"form-encoded", "POST", "application/x-www-form-urlencoded", "a=1", 415},
		{"multipart", "POST", "multipart/form-data; boundary=x", "--x--", 415},
		{"body without content type", "POST", "", `{"a":1}`, 415},
		{"garbage content type", "POST", "garbage;;;", `{"a":1}`, 415},
		{"PUT text/plain", "PUT", "text/plain", "x", 415},
		{"PATCH text/plain", "PATCH", "text/plain", "x", 415},
		{"bodiless POST (logout)", "POST", "", "", 200},
		{"bodiless DELETE", "DELETE", "", "", 200},
		{"GET", "GET", "", "", 200},
	}
	for _, c := range cases {
		hdr := map[string]string{}
		if c.ctype != "" {
			hdr["Content-Type"] = c.ctype
		}
		if w := (secReq{method: c.method, body: c.body, headers: hdr}).do(h); w.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.want)
		}
	}
}

// ---- end to end with the real login handler -------------------------------

// Before this change, any web page could make a visitor's browser send
// text/plain "login" POSTs to the app (no CORS preflight needed). Each
// one counted as a failed attempt for the visitor's IP, so three of them
// locked the owner out. Now they never reach the limiter.
func TestCrossSiteLoginAttemptsCannotBurnTheLimiter(t *testing.T) {
	a := newTestAuth(t, "")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.login)
	h := mustBuild(t, mux)

	for i := 0; i < 10; i++ {
		// the classic CSRF form post
		w := secReq{method: "POST", path: "/api/login", body: `{"password":"guess"}`,
			headers: map[string]string{"Content-Type": "text/plain", "Sec-Fetch-Site": "cross-site"}}.do(h)
		if w.Code != 403 {
			t.Fatalf("cross-site text/plain login = %d, want 403", w.Code)
		}
		// even with a JSON content type (a CORS-enabled attacker origin)
		w = secReq{method: "POST", path: "/api/login", body: `{"password":"guess"}`,
			headers: jsonH(map[string]string{"Sec-Fetch-Site": "cross-site"})}.do(h)
		if w.Code != 403 {
			t.Fatalf("cross-site json login = %d, want 403", w.Code)
		}
	}
	if n := len(a.limiter.state); n != 0 {
		t.Fatalf("limiter tracked %d keys; blocked requests must not count as attempts", n)
	}

	same := map[string]string{"Sec-Fetch-Site": "same-origin"}
	if w := (secReq{method: "POST", path: "/api/login", body: `{"password":"wrong"}`, headers: jsonH(same)}).do(h); w.Code != 401 {
		t.Fatalf("legit wrong password = %d, want 401 (handler must be reachable)", w.Code)
	}
	w := secReq{method: "POST", path: "/api/login", body: `{"password":"` + defaultPassword + `"}`, headers: jsonH(same)}.do(h)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Set-Cookie"), sessionCookieName) {
		t.Fatalf("legit login = %d, cookie %q", w.Code, w.Header().Get("Set-Cookie"))
	}
	// Hardened cookie + headers on the login response itself.
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("login response missing hardening headers: %v", w.Header())
	}
}
