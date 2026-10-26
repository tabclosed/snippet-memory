package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// failingStore returns a store with two saved snippets plus helpers to
// make every subsequent save fail (the store directory is replaced by a
// regular file, so MkdirAll errors) and to make saving work again.
func failingStore(t *testing.T) (s *Store, dbFile string, breakIt, fixIt func()) {
	t.Helper()
	root := t.TempDir()
	dbDir := filepath.Join(root, "db")
	dbFile = filepath.Join(dbDir, "snippets.json")
	s, err := NewStore(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"first", "second"} {
		if _, err := s.Create(SnippetInput{Description: c, Content: c, Tags: []string{"t"}}); err != nil {
			t.Fatal(err)
		}
	}
	breakIt = func() {
		os.RemoveAll(dbDir)
		os.WriteFile(dbDir, []byte("blocker"), 0600)
	}
	fixIt = func() { os.Remove(dbDir) }
	return
}

func assertUnchanged(t *testing.T, s *Store, before []Snippet, beforeNextID int, what string) {
	t.Helper()
	if after := s.All(); !reflect.DeepEqual(before, after) {
		t.Errorf("%s: in-memory state changed despite failed save:\nbefore=%+v\nafter =%+v", what, before, after)
	}
	if s.nextID != beforeNextID {
		t.Errorf("%s: nextID %d -> %d despite failed save", what, beforeNextID, s.nextID)
	}
}

func TestFailedSaveLeavesMemoryUntouched(t *testing.T) {
	s, _, breakIt, _ := failingStore(t)
	before, nextID := s.All(), s.nextID
	breakIt()

	if _, err := s.Create(SnippetInput{Content: "new"}); err == nil {
		t.Error("Create: expected error")
	}
	assertUnchanged(t, s, before, nextID, "Create")

	if _, _, err := s.Update(1, SnippetInput{Description: "CHANGED", Content: "CHANGED"}); err == nil {
		t.Error("Update: expected error")
	}
	assertUnchanged(t, s, before, nextID, "Update")

	if _, _, err := s.SetOrder(1, 99); err == nil {
		t.Error("SetOrder: expected error")
	}
	assertUnchanged(t, s, before, nextID, "SetOrder")

	if _, err := s.Delete(1); err == nil {
		t.Error("Delete: expected error")
	}
	assertUnchanged(t, s, before, nextID, "Delete")
}

// The scenario from the audit: a failed change must not sneak onto disk
// with the next successful save.
func TestFailedChangeDoesNotResurfaceOnNextSave(t *testing.T) {
	s, dbFile, breakIt, fixIt := failingStore(t)
	breakIt()
	s.Create(SnippetInput{Description: "ghost", Content: "ghost"})
	s.Update(1, SnippetInput{Description: "ghost-edit", Content: "ghost-edit"})
	s.Delete(2)
	fixIt()

	// A successful, unrelated write...
	if _, err := s.Create(SnippetInput{Description: "real", Content: "real"}); err != nil {
		t.Fatal(err)
	}
	// ...then reload from disk: only the real change should be there.
	reloaded, err := NewStore(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, sn := range reloaded.All() {
		got[sn.Description] = true
		if sn.Description == "ghost-edit" || sn.Description == "ghost" {
			t.Errorf("failed change leaked to disk: %+v", sn)
		}
	}
	for _, want := range []string{"first", "second", "real"} {
		if !got[want] {
			t.Errorf("missing %q on disk; have %v", want, got)
		}
	}
	// "ghost" never got an ID, so "real" reuses it instead of leaving a gap.
	if reloaded.nextID != 4 {
		t.Errorf("nextID = %d, want 4", reloaded.nextID)
	}
}

func TestSuccessfulMutationsStillWork(t *testing.T) {
	s, dbFile, _, _ := failingStore(t)
	if _, ok, err := s.Update(1, SnippetInput{Description: "u", Content: "u"}); !ok || err != nil {
		t.Fatalf("update: %v %v", ok, err)
	}
	if _, ok, err := s.SetOrder(2, 7); !ok || err != nil {
		t.Fatalf("setorder: %v %v", ok, err)
	}
	if ok, err := s.Delete(1); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, _ := s.Delete(1); ok {
		t.Fatal("deleting twice should report not found")
	}
	if _, ok, _ := s.Update(42, SnippetInput{}); ok {
		t.Fatal("updating a missing id should report not found")
	}
	re, err := NewStore(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	all := re.All()
	if len(all) != 1 || all[0].ID != 2 || all[0].Order != 7 {
		t.Fatalf("unexpected persisted state: %+v", all)
	}
	// Memory and disk agree.
	if !reflect.DeepEqual(all, s.All()) {
		t.Fatalf("memory/disk differ:\n%+v\n%+v", all, s.All())
	}
}
