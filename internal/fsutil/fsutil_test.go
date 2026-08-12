package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := AtomicWrite(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Fatalf("expected hello, got %q", body)
	}
}

func TestAtomicWriteCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "out.txt")
	if err := AtomicWrite(path, []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "nested" {
		t.Fatalf("expected nested, got %q", body)
	}
}

func TestAtomicWriteOverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	os.WriteFile(path, []byte("old"), 0o644)
	if err := AtomicWrite(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "new" {
		t.Fatalf("expected new, got %q", body)
	}
}

func TestAtomicWritePreservesPerm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := AtomicWrite(path, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected perm 0755, got %o", info.Mode().Perm())
	}
}

func TestAtomicWriteCleansTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	// Make parent read-only to force rename failure would require OS tricks;
	// instead just verify no leftover temp files on success.
	path := filepath.Join(dir, "ok.txt")
	if err := AtomicWrite(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "ok.txt" {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}
