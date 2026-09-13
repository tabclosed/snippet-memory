package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// defaultPassword seeds the very first run — once auth.json exists, this
// is never consulted again; change the password from the Settings screen
// instead. You can also override just the *initial* password (useful for
// a scripted first-time setup) with:
//
//	SNIPPET_MEMORY_PASSWORD=your-password go run .
const defaultPassword = "pass-me-in"

// hashIterations is the work factor for new/changed passwords: each
// login guess costs this many extra SHA-256 calls instead of one. A
// single SHA-256 call is fast enough that a stolen auth.json could be
// brute-forced quickly on a GPU; iterating it this many times is a
// manual, stdlib-only stand-in for a real KDF (neither bcrypt nor
// argon2 is in Go's standard library). It's better than a single hash,
// but not a substitute for one of those — if you're willing to add a
// dependency, `go get golang.org/x/crypto/bcrypt` is the stronger,
// better-audited choice.
const hashIterations = 200_000

// passwordRecord is what's persisted to disk: a salted hash, never the
// password itself. Iterations records how many extra rounds were used
// (0 for records written before this field existed, i.e. a single plain
// SHA-256 call) so old records can still be verified correctly and are
// then transparently upgraded to the current work factor — see Verify.
type passwordRecord struct {
	Salt       string `json:"salt"` // hex-encoded
	Hash       string `json:"hash"` // hex-encoded
	Iterations int    `json:"iterations"`
}

// PasswordStore holds the single shared login password for the app,
// persisted as a salted hash in a small JSON file (auth.json, living
// next to snippets.json in the database folder).
type PasswordStore struct {
	mu       sync.RWMutex
	filePath string
	record   passwordRecord
}

// NewPasswordStore loads the stored password hash from filePath. If the
// file doesn't exist yet (first run), it's initialized from
// SNIPPET_MEMORY_PASSWORD or defaultPassword and saved.
func NewPasswordStore(filePath string) (*PasswordStore, error) {
	p := &PasswordStore{filePath: filePath}

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		initial := os.Getenv("SNIPPET_MEMORY_PASSWORD")
		if initial == "" {
			initial = defaultPassword
		}
		if err := p.setLocked(initial); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading auth file: %w", err)
	}
	if err := json.Unmarshal(data, &p.record); err != nil {
		return nil, fmt.Errorf("parsing auth file: %w", err)
	}
	return p, nil
}

// hash computes a salted digest of password, iterated `iterations`
// times. iterations=0 reproduces this project's original (weaker)
// single-SHA256-call format, which is what makes verifying old records
// still work correctly.
func hash(password string, salt []byte, iterations int) []byte {
	sum := append([]byte{}, salt...)
	sum = append(sum, []byte(password)...)
	digest := sha256.Sum256(sum)
	for i := 0; i < iterations; i++ {
		digest = sha256.Sum256(digest[:])
	}
	return digest[:]
}

// Verify reports whether password matches the currently stored one. If
// it matches but was hashed with an old, weaker work factor, it's
// transparently re-hashed at the current strength and saved — so
// existing installs upgrade the next time someone logs in, without
// anyone needing to change their password manually or getting locked
// out by the upgrade.
func (p *PasswordStore) Verify(password string) bool {
	p.mu.RLock()
	salt, err1 := hex.DecodeString(p.record.Salt)
	want, err2 := hex.DecodeString(p.record.Hash)
	iterations := p.record.Iterations
	p.mu.RUnlock()

	if err1 != nil || err2 != nil {
		return false
	}

	got := hash(password, salt, iterations)
	ok := subtle.ConstantTimeCompare(got, want) == 1

	if ok && iterations < hashIterations {
		_ = p.Change(password) // best-effort upgrade; login still succeeds even if this fails
	}
	return ok
}

// Change replaces the stored password with newPassword and persists it.
// Generates a fresh random salt each time.
func (p *PasswordStore) Change(newPassword string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.setLocked(newPassword)
}

// setLocked does the actual hashing + persisting. Callers must hold p.mu
// (or, during construction, be the only goroutine with a reference yet).
func (p *PasswordStore) setLocked(password string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}
	sum := hash(password, salt, hashIterations)
	p.record = passwordRecord{
		Salt:       hex.EncodeToString(salt),
		Hash:       hex.EncodeToString(sum),
		Iterations: hashIterations,
	}

	data, err := json.MarshalIndent(p.record, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding auth file: %w", err)
	}
	if dir := filepath.Dir(p.filePath); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating auth directory: %w", err)
		}
	}
	if err := os.WriteFile(p.filePath, data, 0600); err != nil {
		return fmt.Errorf("writing auth file: %w", err)
	}
	return nil
}
