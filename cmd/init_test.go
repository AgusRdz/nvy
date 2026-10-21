package cmd

import (
	"runtime"
	"strings"
	"testing"
)

func TestReloadHint(t *testing.T) {
	got := reloadHint("/p/profile")
	if runtime.GOOS == "windows" {
		if !strings.Contains(got, `. "/p/profile"`) || strings.Contains(got, "source") {
			t.Errorf("reloadHint = %q, want dot-source form", got)
		}
		return
	}
	if !strings.Contains(got, "source /p/profile") {
		t.Errorf("reloadHint = %q, want source form", got)
	}
}
