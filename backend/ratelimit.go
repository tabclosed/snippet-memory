package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Login attempts are rate-limited per client IP with an escalating
// lockout: the default password (pass-me-in) is public knowledge from
// the README, and without this, anyone who can reach the server could
// just script through a wordlist with no penalty.
//
//   - 3 failed attempts -> locked out for 15 minutes
//   - 3 more failed attempts (after that lockout expires) -> 30 minutes
//   - 3 more after that (and every time after) -> 1 hour
//
// A successful login clears the slate entirely — both the failure count
// and the escalation level reset to the beginning.
const loginMaxAttempts = 3

// lockoutDurations[i] is how long the (i+1)th lockout lasts. Past the
// end of this slice, every further lockout uses the last (longest)
// entry — i.e. it caps at 1 hour rather than escalating forever.
var lockoutDurations = []time.Duration{
	15 * time.Minute,
	30 * time.Minute,
	60 * time.Minute,
}

// ipLoginState is one IP's progress toward (and through) lockouts.
type ipLoginState struct {
	failures    int       // failed attempts since the last reset (success, or a lockout ending)
	lockoutTier int       // how many times this IP has been locked out so far
	lockedUntil time.Time // zero value = not currently locked out
}

// loginLimiter tracks login attempts per IP in memory. Deliberately
// simple (a map protected by one mutex) rather than a token-bucket
// library — this is a personal tool, not a public service, so it
// doesn't need to be more than "good enough to stop a naive script."
type loginLimiter struct {
	mu    sync.Mutex
	state map[string]*ipLoginState
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{state: make(map[string]*ipLoginState)}
}

// Allowed reports whether ip is currently allowed to attempt a login,
// and if not, how much longer it's locked out for.
func (l *loginLimiter) Allowed(ip string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	s, exists := l.state[ip]
	if !exists || s.lockedUntil.IsZero() {
		return true, 0
	}

	remaining := time.Until(s.lockedUntil)
	if remaining <= 0 {
		// The lockout has expired — clear it and give this IP a fresh
		// set of attempts before the next (longer) escalation.
		s.lockedUntil = time.Time{}
		s.failures = 0
		return true, 0
	}
	return false, remaining
}

// RecordFailure notes a failed attempt from ip, locking it out (with an
// escalating duration) once it hits loginMaxAttempts.
func (l *loginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	s, exists := l.state[ip]
	if !exists {
		s = &ipLoginState{}
		l.state[ip] = s
	}

	s.failures++
	if s.failures < loginMaxAttempts {
		return
	}

	duration := lockoutDurations[len(lockoutDurations)-1]
	if s.lockoutTier < len(lockoutDurations) {
		duration = lockoutDurations[s.lockoutTier]
	}
	s.lockedUntil = time.Now().Add(duration)
	s.lockoutTier++
	s.failures = 0
}

// RecordSuccess clears ip's history entirely after a successful login —
// both the failure count and how many times it's escalated reset back
// to the start.
func (l *loginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, ip)
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

// clientIP extracts the request's IP, stripping the port. Uses
// RemoteAddr directly rather than trusting X-Forwarded-For — that
// header is attacker-controlled unless you know you're behind a proxy
// that sets it faithfully, and trusting it blindly would let an
// attacker spoof a fresh IP on every request to dodge the rate limit
// entirely, defeating the point of this file.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
