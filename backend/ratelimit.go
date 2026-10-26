package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Login attempts are rate-limited with an escalating lockout: the default
// password (pass-me-in) is public knowledge from the README, and without
// this, anyone who can reach the server could just script through a
// wordlist with no penalty.
//
//   - 3 failed attempts -> locked out for 15 minutes
//   - 3 more failed attempts (after that lockout expires) -> 30 minutes
//   - 3 more after that (and every time after) -> 1 hour
//
// A successful attempt clears the slate entirely — both the failure count
// and the escalation level reset to the beginning.
//
// The same limiter guards the "change password" endpoint (keyed per
// session instead of per IP), because that endpoint also checks a
// password and would otherwise be an unthrottled guessing oracle for
// anyone holding a stolen session cookie.
const loginMaxAttempts = 3

// lockoutDurations[i] is how long the (i+1)th lockout lasts. Past the
// end of this slice, every further lockout uses the last (longest)
// entry — i.e. it caps at 1 hour rather than escalating forever.
var lockoutDurations = []time.Duration{
	15 * time.Minute,
	30 * time.Minute,
	60 * time.Minute,
}

const (
	// stateTTL is how long an idle, not-locked-out key is remembered.
	// Failure counts and escalation level decay after this much quiet.
	stateTTL = 24 * time.Hour
	// pruneInterval bounds how often the O(n) cleanup sweep runs.
	pruneInterval = time.Minute
	// maxTrackedKeys is a hard ceiling on memory. A client that can
	// cycle through addresses (IPv6, botnets) can't grow the map forever.
	maxTrackedKeys = 10000
)

// keyState is one key's progress toward (and through) lockouts.
type keyState struct {
	failures    int       // attempts since the last reset (success, or a lockout starting)
	lockoutTier int       // how many times this key has been locked out so far
	lockedUntil time.Time // zero value = not currently locked out
	lastSeen    time.Time // for pruning idle entries
}

// loginLimiter tracks attempts per key in memory. Deliberately simple (a
// map protected by one mutex) rather than a token-bucket library — this
// is a personal tool, not a public service.
type loginLimiter struct {
	mu        sync.Mutex
	state     map[string]*keyState
	lastPrune time.Time
	now       func() time.Time // injectable for tests
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{state: make(map[string]*keyState), now: time.Now}
}

// Attempt atomically checks AND counts one attempt for key. If the key is
// locked out it returns (false, remaining). Otherwise the attempt is
// counted immediately — before the (slow) password hash runs — and, if it
// is the loginMaxAttempts-th, the lockout starts right away.
//
// Counting up front is what closes the check-then-record race: with the
// old Allowed()+RecordFailure() pair, a burst of parallel requests could
// all pass the check before any failure was recorded. Now exactly
// loginMaxAttempts requests per window can ever reach the password
// check. A successful attempt calls RecordSuccess, which wipes the
// state (including a lockout set by that very attempt).
func (l *loginLimiter) Attempt(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.pruneLocked(now)

	s, exists := l.state[key]
	if !exists {
		l.makeRoomLocked()
		s = &keyState{}
		l.state[key] = s
	}
	s.lastSeen = now

	if !s.lockedUntil.IsZero() {
		if remaining := s.lockedUntil.Sub(now); remaining > 0 {
			return false, remaining
		}
		// The lockout has expired — give this key a fresh set of
		// attempts before the next (longer) escalation.
		s.lockedUntil = time.Time{}
		s.failures = 0
	}

	s.failures++
	if s.failures >= loginMaxAttempts {
		duration := lockoutDurations[len(lockoutDurations)-1]
		if s.lockoutTier < len(lockoutDurations) {
			duration = lockoutDurations[s.lockoutTier]
		}
		s.lockedUntil = now.Add(duration)
		s.lockoutTier++
		s.failures = 0
	}
	return true, 0
}

// RecordSuccess clears key's history entirely after a successful
// attempt — the failure count, any lockout, and the escalation level.
func (l *loginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, key)
}

// pruneLocked drops entries that are idle and no longer locked out, so
// the map doesn't grow without bound. Runs at most once per
// pruneInterval. Callers must hold l.mu.
func (l *loginLimiter) pruneLocked(now time.Time) {
	if now.Sub(l.lastPrune) < pruneInterval {
		return
	}
	l.lastPrune = now
	for k, s := range l.state {
		lockedOut := !s.lockedUntil.IsZero() && s.lockedUntil.After(now)
		if !lockedOut && now.Sub(s.lastSeen) > stateTTL {
			delete(l.state, k)
		}
	}
}

// makeRoomLocked enforces maxTrackedKeys before a new key is added by
// evicting the least-recently-seen entry. Callers must hold l.mu.
func (l *loginLimiter) makeRoomLocked() {
	for len(l.state) >= maxTrackedKeys {
		var oldestKey string
		var oldest time.Time
		first := true
		for k, s := range l.state {
			if first || s.lastSeen.Before(oldest) {
				oldestKey, oldest, first = k, s.lastSeen, false
			}
		}
		delete(l.state, oldestKey)
	}
}

// formatDuration renders a lockout duration in a friendly way for the
// error message ("15 minutes", "1 hour"), rounding to the nearest
// minute rather than showing something like "14 minutes 58 seconds".
func formatDuration(d time.Duration) string {
	minutes := int(d.Round(time.Minute).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	if minutes >= 60 && minutes%60 == 0 {
		hours := minutes / 60
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	if minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", minutes)
}

// ---- Client identification ----------------------------------------------

// parseTrustedProxies parses a comma-separated list of proxy IPs and/or
// CIDR ranges (e.g. "127.0.0.1, 10.0.0.0/8, ::1"). Empty input means no
// trusted proxies.
func parseTrustedProxies(csv string) ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, n, err := net.ParseCIDR(part)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", part, err)
			}
			nets = append(nets, n)
			continue
		}
		ip := net.ParseIP(part)
		if ip == nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: not an IP address or CIDR", part)
		}
		bits := 128
		if ip.To4() != nil {
			ip, bits = ip.To4(), 32
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets, nil
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func stripZone(host string) string {
	if i := strings.IndexByte(host, '%'); i >= 0 {
		return host[:i]
	}
	return host
}

// clientIP returns the address to attribute a request to.
//
// By default it uses RemoteAddr and ignores X-Forwarded-For entirely —
// that header is attacker-controlled, and trusting it blindly would let
// an attacker invent a fresh "IP" on every request and dodge the limiter.
//
// The catch is a reverse proxy (Caddy, nginx, Tailscale serve...): every
// request then arrives from the proxy's address, so all clients share one
// limiter bucket and three bad guesses from anyone lock out the owner.
// To fix that without opening the spoofing hole, the operator lists their
// proxy addresses in SNIPPET_MEMORY_TRUSTED_PROXIES. X-Forwarded-For is
// consulted ONLY when the direct peer is one of those, and it's read
// right-to-left, skipping other trusted proxies, so the result is the
// first address that a trusted proxy actually observed — never a value
// the client injected at the left of the header.
func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(trusted) == 0 {
		return host
	}
	peer := net.ParseIP(stripZone(host))
	if peer == nil || !ipInNets(peer, trusted) {
		return host // not from a trusted proxy: ignore any forwarded headers
	}

	xff := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if xff == "" {
		return host
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(stripZone(strings.TrimSpace(parts[i])))
		if ip == nil {
			return host // malformed header: don't guess
		}
		if !ipInNets(ip, trusted) {
			return ip.String()
		}
	}
	return host
}

// limiterKey turns a client address into the key the limiter tracks.
// IPv6 clients are bucketed by their /64: a single subscriber typically
// controls an entire /64 (2^64 addresses), so keying on the full address
// would let one attacker rotate to a fresh bucket on every request.
func limiterKey(ip string) string {
	parsed := net.ParseIP(stripZone(ip))
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
