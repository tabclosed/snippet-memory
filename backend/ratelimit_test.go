package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- helpers --------------------------------------------------------------

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func limiterWithClock() (*loginLimiter, *fakeClock) {
	c := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := newLoginLimiter()
	l.now = c.Now
	return l, c
}

func newTestAuth(t *testing.T, trusted string) *authAPI {
	t.Helper()
	ps, err := NewPasswordStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	nets, err := parseTrustedProxies(trusted)
	if err != nil {
		t.Fatal(err)
	}
	return &authAPI{passwords: ps, sessions: NewSessionStore(), limiter: newLoginLimiter(), trustedProxies: nets}
}

func doLogin(a *authAPI, remote, xff, password string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(fmt.Sprintf(`{"password":%q}`, password)))
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	a.login(w, r)
	return w
}

// ---- lockout behaviour (sequential) ---------------------------------------

func TestLockoutEscalationAndReset(t *testing.T) {
	l, clock := limiterWithClock()
	const ip = "1.2.3.4"

	burn := func() { // 3 failed attempts
		for i := 0; i < loginMaxAttempts; i++ {
			if ok, _ := l.Attempt(ip); !ok {
				t.Fatalf("attempt %d unexpectedly blocked", i+1)
			}
		}
	}

	for i, want := range []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute, 60 * time.Minute} {
		burn()
		ok, retry := l.Attempt(ip)
		if ok {
			t.Fatalf("round %d: 4th attempt should be locked out", i)
		}
		if retry > want || retry < want-time.Second {
			t.Fatalf("round %d: retryAfter = %v, want ~%v", i, retry, want)
		}
		clock.Advance(want + time.Second)
	}

	// Success wipes everything, including escalation.
	burn()
	l.RecordSuccess(ip) // third attempt was the correct password
	burn()
	_, retry := l.Attempt(ip)
	if retry < 14*time.Minute || retry > 15*time.Minute {
		t.Fatalf("after success, first lockout should be 15m again, got %v", retry)
	}
}

func TestSuccessOnThirdAttemptClearsItsOwnLockout(t *testing.T) {
	l, _ := limiterWithClock()
	l.Attempt("k")
	l.Attempt("k")
	l.Attempt("k") // 3rd: lockout set at reservation time...
	l.RecordSuccess("k")
	if ok, _ := l.Attempt("k"); !ok {
		t.Fatal("a successful 3rd attempt must not leave the user locked out")
	}
}

// ---- race: exactly N guesses per window, even in a parallel burst ----------

func TestParallelBurstGetsExactlyThreeGuesses(t *testing.T) {
	a := newTestAuth(t, "")
	var evaluated, blocked int32
	var wg sync.WaitGroup
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch doLogin(a, "9.9.9.9:1111", "", "wrong").Code {
			case 401:
				atomic.AddInt32(&evaluated, 1)
			case 429:
				atomic.AddInt32(&blocked, 1)
			}
		}()
	}
	wg.Wait()
	if evaluated != loginMaxAttempts || blocked != 300-loginMaxAttempts {
		t.Fatalf("evaluated=%d blocked=%d, want exactly %d evaluated", evaluated, blocked, loginMaxAttempts)
	}
}

func TestMalformedBodyDoesNotConsumeAttempts(t *testing.T) {
	a := newTestAuth(t, "")
	for i := 0; i < 20; i++ {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader("not json"))
		r.RemoteAddr = "5.5.5.5:1"
		w := httptest.NewRecorder()
		a.login(w, r)
		if w.Code != 400 {
			t.Fatalf("got %d, want 400", w.Code)
		}
	}
	if w := doLogin(a, "5.5.5.5:1", "", defaultPassword); w.Code != 200 {
		t.Fatalf("correct login after malformed requests = %d, want 200", w.Code)
	}
}

// ---- change-password endpoint is limited too ------------------------------

func changePw(a *authAPI, token, cur, next string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/settings/password",
		strings.NewReader(fmt.Sprintf(`{"currentPassword":%q,"newPassword":%q}`, cur, next)))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	r.RemoteAddr = "7.7.7.7:1"
	w := httptest.NewRecorder()
	a.changePassword(w, r)
	return w
}

func TestChangePasswordIsRateLimited(t *testing.T) {
	a := newTestAuth(t, "")
	token, _ := a.sessions.Create()

	for i := 0; i < loginMaxAttempts; i++ {
		if w := changePw(a, token, "wrong", "newpass1"); w.Code != 401 {
			t.Fatalf("guess %d = %d, want 401", i+1, w.Code)
		}
	}
	// Even the CORRECT current password is refused while locked out —
	// otherwise the lockout would only slow down, not stop, a guesser.
	if w := changePw(a, token, defaultPassword, "newpass1"); w.Code != 429 {
		t.Fatalf("after lockout = %d, want 429", w.Code)
	}
	if !a.passwords.Verify(defaultPassword) {
		t.Fatal("password changed during lockout")
	}
}

func TestChangePasswordParallelBurst(t *testing.T) {
	a := newTestAuth(t, "")
	token, _ := a.sessions.Create()
	var evaluated int32
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if changePw(a, token, "wrong", "newpass1").Code == 401 {
				atomic.AddInt32(&evaluated, 1)
			}
		}()
	}
	wg.Wait()
	if evaluated != loginMaxAttempts {
		t.Fatalf("evaluated %d guesses, want %d", evaluated, loginMaxAttempts)
	}
}

func TestChangePasswordHappyPathAndShortPassword(t *testing.T) {
	a := newTestAuth(t, "")
	token, _ := a.sessions.Create()
	// Too-short new password is a 400 and must not burn guesses.
	for i := 0; i < 10; i++ {
		if w := changePw(a, token, "whatever", "abc"); w.Code != 400 {
			t.Fatalf("short password = %d, want 400", w.Code)
		}
	}
	if w := changePw(a, token, defaultPassword, "brand-new-pw"); w.Code != 200 {
		t.Fatalf("valid change = %d, want 200", w.Code)
	}
	if !a.passwords.Verify("brand-new-pw") {
		t.Fatal("new password not in effect")
	}
	if a.sessions.Valid(token) {
		t.Fatal("sessions should be cleared after a password change")
	}
}

// ---- memory is bounded ----------------------------------------------------

func TestIdleEntriesArePruned(t *testing.T) {
	l, clock := limiterWithClock()
	for i := 0; i < 5000; i++ {
		l.Attempt(fmt.Sprintf("10.0.%d.%d", i/250, i%250))
	}
	if len(l.state) != 5000 {
		t.Fatalf("setup: %d entries", len(l.state))
	}
	clock.Advance(stateTTL + 2*pruneInterval)
	l.Attempt("fresh")
	if len(l.state) != 1 {
		t.Fatalf("after TTL, %d entries remain, want 1", len(l.state))
	}
}

func TestLockedEntriesSurvivePruneUntilExpiry(t *testing.T) {
	l, clock := limiterWithClock()
	for i := 0; i < loginMaxAttempts; i++ {
		l.Attempt("locked")
	}
	clock.Advance(10 * time.Minute) // < 15m lockout
	l.Attempt("other")
	if ok, _ := l.Attempt("locked"); ok {
		t.Fatal("lockout lost during prune")
	}
}

func TestMapNeverExceedsHardCap(t *testing.T) {
	l, _ := limiterWithClock()
	for i := 0; i < maxTrackedKeys+500; i++ {
		l.Attempt(fmt.Sprintf("k%d", i))
	}
	if len(l.state) > maxTrackedKeys {
		t.Fatalf("map grew to %d, cap is %d", len(l.state), maxTrackedKeys)
	}
}

// ---- IPv6 bucketing -------------------------------------------------------

func TestLimiterKey(t *testing.T) {
	same := [][2]string{
		{"2001:db8:1:2::1", "2001:db8:1:2:ffff:ffff:ffff:ffff"}, // same /64
		{"::ffff:1.2.3.4", "1.2.3.4"},                           // v4-mapped
	}
	for _, p := range same {
		if limiterKey(p[0]) != limiterKey(p[1]) {
			t.Errorf("%s and %s should share a bucket (%s vs %s)", p[0], p[1], limiterKey(p[0]), limiterKey(p[1]))
		}
	}
	diff := [][2]string{
		{"2001:db8:1:2::1", "2001:db8:1:3::1"}, // different /64
		{"1.2.3.4", "1.2.3.5"},
	}
	for _, p := range diff {
		if limiterKey(p[0]) == limiterKey(p[1]) {
			t.Errorf("%s and %s should NOT share a bucket", p[0], p[1])
		}
	}
	if limiterKey("not-an-ip") != "not-an-ip" {
		t.Error("unparseable input should pass through")
	}
}

// ---- client IP / reverse proxy -------------------------------------------

func TestClientIP(t *testing.T) {
	trusted, err := parseTrustedProxies("10.0.0.1, 192.168.0.0/16, ::1")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote, xff, trustedCfg, want string
	}{
		{"no proxies configured ignores XFF (anti-spoof)", "8.8.8.8:1", "1.1.1.1", "", "8.8.8.8"},
		{"untrusted peer's XFF ignored", "8.8.8.8:1", "1.1.1.1", "x", "8.8.8.8"},
		{"trusted proxy: use forwarded client", "10.0.0.1:1", "203.0.113.7", "x", "203.0.113.7"},
		{"client-injected left entry is skipped", "10.0.0.1:1", "6.6.6.6, 203.0.113.7", "x", "203.0.113.7"},
		{"chain of trusted proxies", "10.0.0.1:1", "203.0.113.7, 192.168.1.5", "x", "203.0.113.7"},
		{"trusted peer, no XFF", "10.0.0.1:1", "", "x", "10.0.0.1"},
		{"malformed XFF falls back to peer", "10.0.0.1:1", "garbage", "x", "10.0.0.1"},
		{"IPv6 trusted loopback", "[::1]:5", "2001:db8::9", "x", "2001:db8::9"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		var tr []*net.IPNet
		if c.trustedCfg != "" {
			tr = trusted
		}
		if got := clientIP(r, tr); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestParseTrustedProxiesErrors(t *testing.T) {
	for _, bad := range []string{"nonsense", "10.0.0.0/99", "1.2.3"} {
		if _, err := parseTrustedProxies(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if n, err := parseTrustedProxies(" , "); err != nil || len(n) != 0 {
		t.Errorf("empty list: %v %v", n, err)
	}
}

// The scenario from the audit: behind a proxy, an attacker locking
// themselves out must not lock out the owner.
func TestProxyAttackerDoesNotLockOutOwner(t *testing.T) {
	const proxy = "127.0.0.1:4000"

	// Without trusted-proxy config: the DoS exists (documenting why the
	// setting is needed).
	naive := newTestAuth(t, "")
	for i := 0; i < loginMaxAttempts; i++ {
		doLogin(naive, proxy, "6.6.6.6", "guess")
	}
	if w := doLogin(naive, proxy, "203.0.113.50", defaultPassword); w.Code != 429 {
		t.Fatalf("expected shared-bucket lockout without config, got %d", w.Code)
	}

	// With it: attacker is locked, owner (different forwarded IP) is fine.
	a := newTestAuth(t, "127.0.0.1")
	for i := 0; i < loginMaxAttempts; i++ {
		doLogin(a, proxy, "6.6.6.6", "guess")
	}
	if w := doLogin(a, proxy, "6.6.6.6", "guess"); w.Code != 429 {
		t.Fatalf("attacker should be locked out, got %d", w.Code)
	}
	if w := doLogin(a, proxy, "203.0.113.50", defaultPassword); w.Code != 200 {
		t.Fatalf("owner should still be able to log in, got %d", w.Code)
	}
	// Attacker can't dodge the lockout by forging a fresh left-hand
	// XFF entry: the proxy appends the real address on the right.
	if w := doLogin(a, proxy, "1.1.1.1, 6.6.6.6", "guess"); w.Code != 429 {
		t.Fatalf("spoofed XFF prefix bypassed the lockout: %d", w.Code)
	}
}
