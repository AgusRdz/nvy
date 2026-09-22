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
