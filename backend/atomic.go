package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces the file at path with data so that a crash or
// power loss at any point leaves either the complete old file or the
// complete new file on disk — never a truncated mix of both.
//
// os.WriteFile truncates the destination first and then writes into it,
// so an interruption mid-write leaves a half-written file (and for
// snippets.json, a server that refuses to start). Instead we write to a
// temp file in the SAME directory (rename is only atomic within one
// filesystem), fsync it, then rename it over the destination.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close() // no-op if already closed
			os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	// Flush to stable storage BEFORE the rename; otherwise the rename can
	// hit the disk first and a crash leaves an empty/partial file under
	// the real name.
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	// CreateTemp always uses 0600; apply the permissions the caller asked
	// for (e.g. 0644 for config.json).
	if err = os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing file: %w", err)
	}

	// Best effort: fsync the directory so the rename itself survives a
	// crash. Not supported on Windows, so errors are ignored.
	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
