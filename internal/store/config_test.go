package store

import "testing"

func TestValidateCheckIntervalHours(t *testing.T) {
	cases := []struct {
		n       int
		wantErr bool
	}{
		{1, false}, {2, false}, {3, false}, {4, false},
		{6, false}, {8, false}, {12, false}, {24, false},
		{0, true}, {-4, true}, {5, true}, {7, true}, {10, true}, {16, true}, {25, true}, {48, true},
	}
	for _, c := range cases {
		err := ValidateCheckIntervalHours(c.n)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateCheckIntervalHours(%d) error = %v, wantErr %v", c.n, err, c.wantErr)
		}
	}
}

func TestCheckInterval(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want int
	}{
		{"unset defaults to 4", Config{}, 4},
		{"default config", DefaultConfig(), 4},
		{"explicit 1", Config{CheckIntervalHours: 1}, 1},
		{"explicit 24", Config{CheckIntervalHours: 24}, 24},
		{"hand-edited invalid falls back", Config{CheckIntervalHours: 48}, 4},
		{"hand-edited uneven falls back", Config{CheckIntervalHours: 5}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.CheckInterval(); got != c.want {
				t.Errorf("CheckInterval() = %d, want %d", got, c.want)
			}
		})
	}
}
