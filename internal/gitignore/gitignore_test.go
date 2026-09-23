package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func read(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(data)
}

func TestEnsureCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := read(t, dir); got != ".env\n.env.nvy\n" {
		t.Errorf("created content mismatch: %q", got)
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	first := read(t, dir)
	if err := Ensure(dir); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if second := read(t, dir); second != first {
		t.Errorf("Ensure not idempotent:\n1st %q\n2nd %q", first, second)
	}
}

func TestEnsurePreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	seed := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(seed), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	want := "node_modules/\ndist/\n.env\n.env.nvy\n"
	if got := read(t, dir); got != want {
		t.Errorf("existing content not preserved:\nwant %q\n got %q", want, got)
	}
}

func TestEnsureInsertsSeparatorNewline(t *testing.T) {
	// A .gitignore with no trailing newline must not have entries welded onto
	// its last line.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("dist/"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	want := "dist/\n.env\n.env.nvy\n"
	if got := read(t, dir); got != want {
		t.Errorf("separator not inserted:\nwant %q\n got %q", want, got)
	}
}

func TestEnsureAddsOnlyMissingEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	want := ".env\n.env.nvy\n"
	if got := read(t, dir); got != want {
		t.Errorf("partial add mismatch:\nwant %q\n got %q", want, got)
	}
}

func TestEnsureMatchesFullLineOnly(t *testing.T) {
	// A substring match (".environment") must not be mistaken for the ".env"
	// entry — otherwise the real ignore rule never gets added.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".environment\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	want := ".environment\n.env\n.env.nvy\n"
	if got := read(t, dir); got != want {
		t.Errorf("substring falsely matched:\nwant %q\n got %q", want, got)
	}
}
