package cmd

import "testing"

func TestShouldNotify(t *testing.T) {
	cases := []struct {
		name         string
		lastNotified string
		today        string
		want         bool
	}{
		{"never notified", "", "2026-09-22", true},
		{"notified today", "2026-09-22", "2026-09-22", false},
		{"notified yesterday", "2026-09-21", "2026-09-22", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldNotify(c.lastNotified, c.today)
			if got != c.want {
				t.Errorf("shouldNotify(%q, %q) = %v, want %v", c.lastNotified, c.today, got, c.want)
			}
		})
	}
}
