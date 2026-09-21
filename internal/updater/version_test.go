package updater

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.1.0", "v0.0.9", true},
		{"v0.2.0", "v0.1.9", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.1.0", "v0.1.0", false}, // equal is not newer
		{"v0.0.9", "v0.1.0", false}, // older is not newer
		{"dev", "v0.1.0", false},    // dev is never newer
		{"v0.1.0", "dev", false},    // dev is never a valid comparator either
	}

	for _, c := range cases {
		got := IsNewer(c.a, c.b)
		if got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewerMalformed(t *testing.T) {
	if IsNewer("not-a-version", "v0.1.0") {
		t.Error("IsNewer with malformed input should be false")
	}
	if IsNewer("v0.1.0", "not-a-version") {
		t.Error("IsNewer with malformed input should be false")
	}
}

func TestIsDev(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"v1.2.3-dirty", true},
		{"v1.2.3", false},
		{"v0.1.0", false},
	}
	for _, c := range cases {
		if got := IsDev(c.version); got != c.want {
			t.Errorf("IsDev(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}
