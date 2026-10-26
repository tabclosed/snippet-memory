package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store manages reading and writing snippets to a JSON file on disk.
// A sync.RWMutex guards access so concurrent HTTP requests (Go's server
// handles each request on its own goroutine) don't corrupt the file or
// race on the in-memory slice.
type Store struct {
	mu       sync.RWMutex
	filePath string
	snippets []Snippet
	nextID   int
}

// NewStore loads snippets from filePath into memory. If the file doesn't
// exist yet, it starts with an empty list (the file gets created on first
// write).
func NewStore(filePath string) (*Store, error) {
	s := &Store{filePath: filePath, nextID: 1}

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading store file: %w", err)
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.snippets); err != nil {
			return nil, fmt.Errorf("parsing store file: %w", err)
		}
	}

	// Data saved before the Tags field existed (or written by hand)
	// might have "tags" missing or explicitly null. Go's json package
	// leaves that as a nil slice, which marshals right back out as
	// `"tags": null` — and `null.map(...)` throws in the frontend.
	// Normalize to an empty slice so API responses always give the
	// frontend a real (possibly empty) array to work with.
	for i := range s.snippets {
		if s.snippets[i].Tags == nil {
			s.snippets[i].Tags = []string{}
		}
	}

	for _, snip := range s.snippets {
		if snip.ID >= s.nextID {
			s.nextID = snip.ID + 1
		}
	}

	// Migrate data saved before the Order field existed. Heuristic: if
	// every snippet has Order == 0 and there's more than one of them,
	// this is legacy data rather than a real (impossible) tie — assign
	// order based on creation time, newest first, matching the sort this
	// store always used before Order existed, then persist it once so
	// this only runs the first time.
	needsMigration := len(s.snippets) > 1
	for _, snip := range s.snippets {
		if snip.Order != 0 {
			needsMigration = false
			break
		}
	}
	if needsMigration {
		sort.Slice(s.snippets, func(i, j int) bool {
			return s.snippets[i].CreatedAt.After(s.snippets[j].CreatedAt)
		})
		for i := range s.snippets {
			s.snippets[i].Order = i
		}
		if err := s.save(s.snippets); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// save writes the given snippets to disk as JSON. It deliberately takes
// the slice to persist rather than reading s.snippets, so mutators can
// write a modified COPY first and only adopt it in memory once the write
// has succeeded (see commit). Callers must hold s.mu (write lock).
func (s *Store) save(snippets []Snippet) error {
	data, err := json.MarshalIndent(snippets, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding snippets: %w", err)
	}
	// Create the parent directory if it doesn't exist yet — matters most
	// for the standalone .exe build, where the "database" folder next to
	// the executable won't exist on a machine's first run. 0700/0600
	// (owner-only) rather than the more common 0755/0644, since snippets
	// can easily contain secrets/credentials people paste into a
	// snippet manager — other local accounts on a shared machine
	// shouldn't be able to read them, matching auth.json's permissions.
	if dir := filepath.Dir(s.filePath); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating store directory: %w", err)
		}
	}
	if err := writeFileAtomic(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("writing store file: %w", err)
	}
	return nil
}

// All returns every snippet, ordered by Order ascending (i.e. the order
// the up/down reorder buttons put them in — new snippets default to the
// top; see Create).
func (s *Store) All() []Snippet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Snippet, len(s.snippets))
	copy(out, s.snippets)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Order < out[j].Order
	})
	return out
}

// Get returns a single snippet by ID.
func (s *Store) Get(id int) (Snippet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, snip := range s.snippets {
		if snip.ID == id {
			return snip, true
		}
	}
	return Snippet{}, false
}

// normalizeTags ensures a snippet's Tags is always a real (possibly
// empty) slice, never nil — a nil slice marshals to JSON `null`, and
// `null.map(...)` throws in the frontend.
func normalizeTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

// commit persists next and, only if that succeeds, makes it the store's
// in-memory state. If the write fails, s.snippets and s.nextID are left
// exactly as they were, so memory never gets ahead of disk: the caller's
// 500 means "nothing changed", and a later successful save can't
// resurrect a change the client was told had failed.
//
// Every mutator follows the same copy-on-write pattern: clone the slice,
// modify the clone, commit. Callers must hold s.mu (write lock).
func (s *Store) commit(next []Snippet, nextID int) error {
	if err := s.save(next); err != nil {
		return err
	}
	s.snippets = next
	s.nextID = nextID
	return nil
}

// cloneSnippets returns a copy of the slice that can be modified without
// touching the original. Snippet values are copied; the Tags slices are
// shared, which is safe because mutators replace Tags wholesale
// (normalizeTags(input.Tags)) and never edit them in place.
func cloneSnippets(in []Snippet) []Snippet {
	out := make([]Snippet, len(in))
	copy(out, in)
	return out
}

// Create adds a new snippet and persists it to disk. New snippets are
// inserted at the top of the display order (Order 0) — every existing
// snippet's Order shifts down by one to make room.
func (s *Store) Create(input SnippetInput) (Snippet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make([]Snippet, len(s.snippets), len(s.snippets)+1)
	copy(next, s.snippets)
	for i := range next {
		next[i].Order++
	}

	now := time.Now().UTC()
	snip := Snippet{
		ID:          s.nextID,
		Description: input.Description,
		Language:    input.Language,
		Tags:        normalizeTags(input.Tags),
		Content:     input.Content,
		Order:       0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	next = append(next, snip)

	// The ID is only consumed if the write succeeds.
	if err := s.commit(next, s.nextID+1); err != nil {
		return Snippet{}, err
	}
	return snip, nil
}

// Update replaces an existing snippet's editable fields and persists it.
func (s *Store) Update(id int, input SnippetInput) (Snippet, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, snip := range s.snippets {
		if snip.ID == id {
			snip.Description = input.Description
			snip.Language = input.Language
			snip.Tags = normalizeTags(input.Tags)
			snip.Content = input.Content
			snip.UpdatedAt = time.Now().UTC()

			next := cloneSnippets(s.snippets)
			next[i] = snip
			if err := s.commit(next, s.nextID); err != nil {
				return Snippet{}, false, err
			}
			return snip, true, nil
		}
	}
	return Snippet{}, false, nil
}

// SetOrder updates just a single snippet's display-order value and
// persists it. Used by the up/down reorder buttons: the frontend swaps
// two adjacent snippets' Order values with two calls to this.
func (s *Store) SetOrder(id, order int) (Snippet, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, snip := range s.snippets {
		if snip.ID == id {
			snip.Order = order

			next := cloneSnippets(s.snippets)
			next[i] = snip
			if err := s.commit(next, s.nextID); err != nil {
				return Snippet{}, false, err
			}
			return snip, true, nil
		}
	}
	return Snippet{}, false, nil
}

// Delete removes a snippet by ID and persists the change.
func (s *Store) Delete(id int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, snip := range s.snippets {
		if snip.ID == id {
			// Build a fresh slice instead of append(s.snippets[:i], ...),
			// which shifts elements inside the live backing array and
			// would corrupt in-memory state if the save then failed.
			next := make([]Snippet, 0, len(s.snippets)-1)
			next = append(next, s.snippets[:i]...)
			next = append(next, s.snippets[i+1:]...)
			if err := s.commit(next, s.nextID); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}
