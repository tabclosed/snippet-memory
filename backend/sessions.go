package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// sessionTTL is how long a login lasts before needing to sign in again.
const sessionTTL = 30 * 24 * time.Hour

// SessionStore tracks logged-in sessions in memory. Deliberately not
// persisted to disk: if the server restarts, everyone just logs back in
// with the password — simpler than dealing with session persistence for
// what's a single-shared-password, mostly-single-user tool.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]time.Time)}
}

// Create generates a new session token and remembers it until it expires.
func (s *SessionStore) Create() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)

	s.mu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.mu.Unlock()

	return token, nil
}

// Valid reports whether token is a known, unexpired session.
func (s *SessionStore) Valid(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(s.sessions, token)
		return false
	}
	return true
}

// Delete removes a session (logout).
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// Clear invalidates every session — used after a password change, so a
// cookie stolen before the change stops working immediately rather than
// staying valid until it naturally expires.
func (s *SessionStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]time.Time)
}
