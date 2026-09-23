//go:build windows

package platform

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestSplitPathValue(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"C:\\a;C:\\b", []string{"C:\\a", "C:\\b"}},
		{" C:\\a ; ; C:\\b ", []string{"C:\\a", "C:\\b"}}, // trims + drops empties
		{"C:\\only", []string{"C:\\only"}},
	}
	for _, c := range cases {
		if got := splitPathValue(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitPathValue(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestAddPathEntry(t *testing.T) {
	got, err := addPathEntry([]string{"C:\\a"}, "C:\\b")
	if err != nil {
		t.Fatalf("addPathEntry: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"C:\\a", "C:\\b"}) {
		t.Errorf("append mismatch: %#v", got)
	}
	// Case-insensitive duplicate is rejected.
	if _, err := addPathEntry([]string{"C:\\Tools"}, "c:\\tools"); err == nil {
		t.Error("expected error on case-insensitive duplicate")
	}
}

func TestRemovePathEntry(t *testing.T) {
	got, err := removePathEntry([]string{"C:\\a", "C:\\b"}, "c:\\A")
	if err != nil {
		t.Fatalf("removePathEntry: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"C:\\b"}) {
		t.Errorf("remove mismatch: %#v", got)
	}
	if _, err := removePathEntry([]string{"C:\\a"}, "C:\\nope"); err == nil {
		t.Error("expected error removing absent entry")
	}
}

// useScratchEnv points envSubkey at a throwaway HKCU key for the duration of the
// test, so registry writers never touch the developer's real Environment. The
// key is created fresh and deleted on cleanup.
func useScratchEnv(t *testing.T) {
	t.Helper()
	path := fmt.Sprintf(`Software\nvy-test-%d`, os.Getpid())
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatalf("create scratch key: %v", err)
	}
	k.Close()

	prev := envSubkey
	envSubkey = path
	t.Cleanup(func() {
		envSubkey = prev
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Logf("cleanup: delete scratch key %s: %v", path, err)
		}
	})
}

func TestGlobalVarRoundTrip(t *testing.T) {
	useScratchEnv(t)
	p := &windowsPlatform{}

	if err := p.ApplyGlobalVar("NVY_RT", "v1"); err != nil {
		t.Fatalf("ApplyGlobalVar: %v", err)
	}
	vars, err := p.ExternalVars()
	if err != nil {
		t.Fatalf("ExternalVars: %v", err)
	}
	if vars["NVY_RT"] != "v1" {
		t.Fatalf("after set, got %q want v1", vars["NVY_RT"])
	}

	// Overwrite.
	if err := p.ApplyGlobalVar("NVY_RT", "v2"); err != nil {
		t.Fatalf("ApplyGlobalVar overwrite: %v", err)
	}
	vars, _ = p.ExternalVars()
	if vars["NVY_RT"] != "v2" {
		t.Fatalf("after overwrite, got %q want v2", vars["NVY_RT"])
	}

	// Remove, then removing again is a no-op (not an error).
	if err := p.RemoveGlobalVar("NVY_RT"); err != nil {
		t.Fatalf("RemoveGlobalVar: %v", err)
	}
	vars, _ = p.ExternalVars()
	if _, ok := vars["NVY_RT"]; ok {
		t.Fatalf("NVY_RT still present after removal")
	}
	if err := p.RemoveGlobalVar("NVY_RT"); err != nil {
		t.Fatalf("RemoveGlobalVar on absent key should be nil, got %v", err)
	}
}

func TestExternalVarsSkipsPath(t *testing.T) {
	useScratchEnv(t)
	p := &windowsPlatform{}
	if err := p.AddToPath(`C:\tools`); err != nil {
		t.Fatalf("AddToPath: %v", err)
	}
	if err := p.ApplyGlobalVar("NVY_REAL", "x"); err != nil {
		t.Fatalf("ApplyGlobalVar: %v", err)
	}
	vars, err := p.ExternalVars()
	if err != nil {
		t.Fatalf("ExternalVars: %v", err)
	}
	if _, ok := vars["Path"]; ok {
		t.Error("ExternalVars must not surface Path — the path command owns it")
	}
	if vars["NVY_REAL"] != "x" {
		t.Errorf("real var missing: %#v", vars)
	}
}

func TestPathRoundTrip(t *testing.T) {
	useScratchEnv(t)
	p := &windowsPlatform{}

	// No Path value yet → empty slice, no error.
	got, err := p.GetPath()
	if err != nil {
		t.Fatalf("GetPath (empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty PATH, got %#v", got)
	}

	if err := p.AddToPath(`C:\tools`); err != nil {
		t.Fatalf("AddToPath tools: %v", err)
	}
	if err := p.AddToPath(`C:\bin`); err != nil {
		t.Fatalf("AddToPath bin: %v", err)
	}
	got, _ = p.GetPath()
	if !reflect.DeepEqual(got, []string{`C:\tools`, `C:\bin`}) {
		t.Fatalf("after adds, got %#v", got)
	}

	// Case-insensitive duplicate rejected, PATH unchanged.
	if err := p.AddToPath(`c:\TOOLS`); err == nil {
		t.Error("expected duplicate error")
	}
	got, _ = p.GetPath()
	if len(got) != 2 {
		t.Errorf("duplicate must not mutate PATH: %#v", got)
	}

	// Case-insensitive removal.
	if err := p.RemoveFromPath(`C:\TOOLS`); err != nil {
		t.Fatalf("RemoveFromPath: %v", err)
	}
	got, _ = p.GetPath()
	if !reflect.DeepEqual(got, []string{`C:\bin`}) {
		t.Fatalf("after remove, got %#v", got)
	}

	// Removing an absent entry errors.
	if err := p.RemoveFromPath(`C:\nope`); err == nil {
		t.Error("expected error removing absent PATH entry")
	}
}
