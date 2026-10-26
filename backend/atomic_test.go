package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")

	if err := writeFileAtomic(p, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(p, []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "two" {
		t.Fatalf("got %q", got)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0600 {
			t.Fatalf("perm = %v", fi.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover temp files: %v", entries)
	}
}

// A failed write must leave the original file untouched and clean up.
func TestWriteFileAtomicFailureKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")
	os.WriteFile(p, []byte("original"), 0600)

	// Destination is a directory -> rename fails.
	bad := filepath.Join(dir, "sub")
	os.Mkdir(bad, 0700)
	os.WriteFile(filepath.Join(bad, "x"), []byte("x"), 0600)
	if err := writeFileAtomic(bad, []byte("new"), 0600); err == nil {
		t.Fatal("expected error")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "original" {
		t.Fatalf("original modified: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "f.json" && e.Name() != "sub" {
			t.Fatalf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestStoreRoundTripAtomic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db", "snippets.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(SnippetInput{Content: "a"}); err != nil {
		t.Fatal(err)
	}
	s2, err := NewStore(p)
	if err != nil || len(s2.All()) != 1 {
		t.Fatalf("reload failed: %v", err)
	}
}
