package cmd

import (
	"runtime"
	"testing"

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

func TestSetGlobalVarPreservesRenewURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("setGlobalVar writes the real user environment on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	const url = "https://gitlab.com/-/user_settings/personal_access_tokens"
	setRenewURL = url
	t.Cleanup(func() { setRenewURL = "" })
	if err := setGlobalVar("TOK", "old", nil, true); err != nil {
		t.Fatal(err)
	}

	// re-set without --renew-url keeps it
	if err := setGlobalVar("TOK", "new", nil, false); err != nil {
		t.Fatal(err)
	}
	gs, err := store.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if gs["TOK"].Value != "new" || gs["TOK"].RenewURL != url {
		t.Errorf("after re-set: %+v, want value=new renew=%s", gs["TOK"], url)
	}

	// explicit empty clears it
	setRenewURL = ""
	if err := setGlobalVar("TOK", "newer", nil, true); err != nil {
		t.Fatal(err)
	}
	gs, _ = store.LoadGlobal()
	if gs["TOK"].RenewURL != "" {
		t.Errorf("renew URL not cleared: %q", gs["TOK"].RenewURL)
	}
}

func TestSetLocalVarPreservesRenewURL(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	const url = "https://example.com/renew"
	setRenewURL = url
	t.Cleanup(func() {
		setRenewURL, setExpires, setNote = "", "", ""
	})
	if err := setLocalVar("TOK", "old", nil, true); err != nil {
		t.Fatal(err)
	}

	// re-set with a note but no --renew-url keeps the URL
	setRenewURL, setNote = "", "rotated"
	if err := setLocalVar("TOK", "new", nil, false); err != nil {
		t.Fatal(err)
	}
	meta, err := store.LoadLocalMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if meta["TOK"].RenewURL != url || meta["TOK"].Note != "rotated" {
		t.Errorf("after re-set: %+v, want renew=%s note=rotated", meta["TOK"], url)
	}

	// re-set with neither flag leaves meta untouched
	setNote = ""
	if err := setLocalVar("TOK", "newest", nil, false); err != nil {
		t.Fatal(err)
	}
	meta, _ = store.LoadLocalMeta(dir)
	if meta["TOK"].RenewURL != url {
		t.Errorf("renew URL lost on plain re-set: %+v", meta["TOK"])
	}
}
