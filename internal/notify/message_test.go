package notify

import "testing"

func TestExpiryMessage(t *testing.T) {
	cases := []struct {
		name string
		days int
		want string
	}{
		{"expired", -1, "FOO has already expired"},
		{"expired further", -30, "FOO has already expired"},
		{"today", 0, "FOO expires today"},
		{"tomorrow", 1, "FOO expires tomorrow"},
		{"in days", 5, "FOO expires in 5 days"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpiryMessage("FOO", c.days)
			if got != c.want {
				t.Errorf("ExpiryMessage(%q, %d) = %q, want %q", "FOO", c.days, got, c.want)
			}
		})
	}
}

func TestRenewHint(t *testing.T) {
	if got := RenewHint("FOO", "https://example.com/renew"); got != " — run: nvy renew FOO" {
		t.Errorf("RenewHint with URL = %q", got)
	}
	if got := RenewHint("FOO", ""); got != "" {
		t.Errorf("RenewHint without URL = %q, want empty", got)
	}
}
