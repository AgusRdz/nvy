package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeEnv seeds dir/.env with content and returns dir.
func writeEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}
	return dir
}

// readEnv returns the raw bytes of dir/.env.
func readEnv(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	return string(data)
}

func TestParseRenderRoundTrip(t *testing.T) {
	// Comments, blank lines, and order must survive a parse→render cycle
	// untouched — this is the "never break an existing .env" invariant.
	in := "# top comment\nA=1\n\nB=two\n# mid\nC=3\n"
	got := renderEnvLines(parseEnvLines(in))
	if got != in {
		t.Errorf("round-trip changed content:\n in: %q\nout: %q", in, got)
	}
}

func TestSetLocalVarUpdatesInPlace(t *testing.T) {
	dir := writeEnv(t, "# keep me\nA=1\n\nB=2\nC=3\n")
	if err := SetLocalVar(dir, "B", "changed"); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	want := "# keep me\nA=1\n\nB=changed\nC=3\n"
	if got := readEnv(t, dir); got != want {
		t.Errorf("in-place update broke layout:\nwant %q\n got %q", want, got)
	}
}

func TestSetLocalVarAppendsNewKey(t *testing.T) {
	dir := writeEnv(t, "A=1\nB=2\n")
	if err := SetLocalVar(dir, "C", "3"); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	want := "A=1\nB=2\nC=3\n"
	if got := readEnv(t, dir); got != want {
		t.Errorf("append mismatch:\nwant %q\n got %q", want, got)
	}
}

func TestSetLocalVarCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := SetLocalVar(dir, "A", "1"); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	if got := readEnv(t, dir); got != "A=1\n" {
		t.Errorf("create mismatch: got %q", got)
	}
}

func TestSetLocalVarNoTrailingNewline(t *testing.T) {
	// A file without a trailing newline must not concatenate the new key onto
	// the last line — it normalizes to one key per line.
	dir := writeEnv(t, "A=1")
	if err := SetLocalVar(dir, "B", "2"); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	if got := readEnv(t, dir); got != "A=1\nB=2\n" {
		t.Errorf("got %q", got)
	}
}

func TestSetLocalVarValueWithEquals(t *testing.T) {
	// Only the first '=' delimits key/value; the rest is part of the value.
	dir := t.TempDir()
	if err := SetLocalVar(dir, "CONN", "a=b;c=d"); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	if got := readEnv(t, dir); got != "CONN=a=b;c=d\n" {
		t.Errorf("got %q", got)
	}
	vars, err := LoadEnv(dir)
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if vars["CONN"] != "a=b;c=d" {
		t.Errorf("value with '=' not preserved: %q", vars["CONN"])
	}
}

func TestSetLocalVarEmptyValue(t *testing.T) {
	dir := writeEnv(t, "A=1\n")
	if err := SetLocalVar(dir, "EMPTY", ""); err != nil {
		t.Fatalf("SetLocalVar: %v", err)
	}
	if got := readEnv(t, dir); got != "A=1\nEMPTY=\n" {
		t.Errorf("got %q", got)
	}
}

func TestRemoveLocalVarPreservesRest(t *testing.T) {
	dir := writeEnv(t, "# header\nA=1\n\nB=2\nC=3\n")
	if err := RemoveLocalVar(dir, "B"); err != nil {
		t.Fatalf("RemoveLocalVar: %v", err)
	}
	want := "# header\nA=1\n\nC=3\n"
	if got := readEnv(t, dir); got != want {
		t.Errorf("remove broke layout:\nwant %q\n got %q", want, got)
	}
}

func TestRemoveLocalVarMissingKeyIsNoop(t *testing.T) {
	dir := writeEnv(t, "A=1\nB=2\n")
	if err := RemoveLocalVar(dir, "ZZZ"); err != nil {
		t.Fatalf("RemoveLocalVar: %v", err)
	}
	if got := readEnv(t, dir); got != "A=1\nB=2\n" {
		t.Errorf("missing-key remove altered file: %q", got)
	}
}

func TestRemoveLocalVarMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveLocalVar(dir, "A"); err != nil {
		t.Fatalf("RemoveLocalVar on missing file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Errorf("remove on missing file should not create .env")
	}
}

func TestLoadEnvIgnoresCommentsAndBlanks(t *testing.T) {
	dir := writeEnv(t, "# comment\n\nA=1\nB=two\n")
	vars, err := LoadEnv(dir)
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if len(vars) != 2 || vars["A"] != "1" || vars["B"] != "two" {
		t.Errorf("unexpected vars: %#v", vars)
	}
}

func TestLoadEnvCRLF(t *testing.T) {
	// CRLF-terminated files (Windows-authored .env) must parse cleanly with no
	// stray carriage returns clinging to values.
	dir := writeEnv(t, "A=1\r\nB=2\r\n")
	vars, err := LoadEnv(dir)
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if vars["A"] != "1" || vars["B"] != "2" {
		t.Errorf("CRLF not stripped: %#v", vars)
	}
}

func TestLocalMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	meta := LocalMetaStore{"API_KEY": {ExpiresAt: &exp, Note: "rotate me"}}
	if err := SaveLocalMeta(dir, meta); err != nil {
		t.Fatalf("SaveLocalMeta: %v", err)
	}
	got, err := LoadLocalMeta(dir)
	if err != nil {
		t.Fatalf("LoadLocalMeta: %v", err)
	}
	entry, ok := got["API_KEY"]
	if !ok {
		t.Fatalf("API_KEY missing after round-trip: %#v", got)
	}
	if entry.ExpiresAt == nil || !entry.ExpiresAt.Equal(exp) || entry.Note != "rotate me" {
		t.Errorf("meta round-trip mismatch: %#v", entry)
	}
}

func TestSaveLocalMetaEmptyDeletesFile(t *testing.T) {
	dir := t.TempDir()
	if err := SaveLocalMeta(dir, LocalMetaStore{"X": {Note: "n"}}); err != nil {
		t.Fatalf("seed SaveLocalMeta: %v", err)
	}
	if err := SaveLocalMeta(dir, LocalMetaStore{}); err != nil {
		t.Fatalf("SaveLocalMeta empty: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env.nvy")); !os.IsNotExist(err) {
		t.Errorf("empty meta should delete .env.nvy sidecar")
	}
}
