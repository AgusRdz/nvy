package cmd

import (
	"runtime"
	"testing"
	"time"

	"github.com/AgusRdz/nvy/internal/store"
)

func TestParseSetArg(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantKey      string
		wantValue    string
		wantHasValue bool
	}{
		{name: "key=value", arg: "FOO=bar", wantKey: "FOO", wantValue: "bar", wantHasValue: true},
		{name: "key=empty value", arg: "FOO=", wantKey: "FOO", wantValue: "", wantHasValue: true},
		{name: "value contains equals", arg: "FOO=a=b", wantKey: "FOO", wantValue: "a=b", wantHasValue: true},
		{name: "bare key", arg: "FOO", wantKey: "FOO", wantValue: "", wantHasValue: false},
		{name: "leading equals", arg: "=FOO", wantKey: "", wantValue: "FOO", wantHasValue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, hasValue := parseSetArg(tt.arg)
			if key != tt.wantKey || value != tt.wantValue || hasValue != tt.wantHasValue {
				t.Errorf("parseSetArg(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.arg, key, value, hasValue, tt.wantKey, tt.wantValue, tt.wantHasValue)
			}
		})
	}
}

func TestSetGlobalVarPreservesMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("setGlobalVar writes the real user environment on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(func() { setRenewURL, setNote = "", "" })

	const url = "https://gitlab.com/-/user_settings/personal_access_tokens"
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	setRenewURL, setNote = url, "ci token"
	if err := setGlobalVar("TOK", "old", &exp, true, true, true); err != nil {
		t.Fatal(err)
	}

	// re-set with no flags keeps expiry, note and renew URL
	setRenewURL, setNote = "", ""
	if err := setGlobalVar("TOK", "new", nil, false, false, false); err != nil {
		t.Fatal(err)
	}
	gs, err := store.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	e := gs["TOK"]
	if e.Value != "new" || e.RenewURL != url || e.Note != "ci token" || e.ExpiresAt == nil || !e.ExpiresAt.Equal(exp) {
		t.Errorf("after re-set: %+v, want value=new with metadata kept", e)
	}

	// explicit flags still clear
	if err := setGlobalVar("TOK", "newer", nil, true, true, true); err != nil {
		t.Fatal(err)
	}
	gs, _ = store.LoadGlobal()
	if e := gs["TOK"]; e.RenewURL != "" || e.Note != "" || e.ExpiresAt != nil {
		t.Errorf("metadata not cleared: %+v", e)
	}
}

func TestSetLocalVarPreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Cleanup(func() { setRenewURL, setNote = "", "" })

	const url = "https://example.com/renew"
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	setRenewURL, setNote = url, "old note"
	if err := setLocalVar("TOK", "old", &exp, true, true, true); err != nil {
		t.Fatal(err)
	}

	// re-set with only --note keeps expiry and renew URL
	setRenewURL, setNote = "", "rotated"
	if err := setLocalVar("TOK", "new", nil, false, true, false); err != nil {
		t.Fatal(err)
	}
	meta, err := store.LoadLocalMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := meta["TOK"]
	if m.RenewURL != url || m.Note != "rotated" || m.ExpiresAt == nil || !m.ExpiresAt.Equal(exp) {
		t.Errorf("after re-set: %+v, want renew=%s note=rotated expiry kept", m, url)
	}

	// re-set with no flags leaves meta untouched
	setNote = ""
	if err := setLocalVar("TOK", "newest", nil, false, false, false); err != nil {
		t.Fatal(err)
	}
	meta, _ = store.LoadLocalMeta(dir)
	if m := meta["TOK"]; m.RenewURL != url || m.Note != "rotated" {
		t.Errorf("metadata lost on plain re-set: %+v", m)
	}
}
