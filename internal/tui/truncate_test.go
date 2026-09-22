package tui

import "testing"

func TestTruncateVisible(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{
			name: "short string returns unchanged",
			in:   "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "exact fit returns unchanged",
			in:   "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "plain string longer than max is cut",
			in:   "HELLO WORLD",
			max:  5,
			want: "HELLO" + "…" + ansiReset,
		},
		{
			name: "color-wrapped string keeps escape codes when truncated",
			in:   cyan("HELLO WORLD"),
			max:  5,
			want: ansiCyan + "HELLO" + "…" + ansiReset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateVisible(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("truncateVisible(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

// TestTruncateVisibleColorByteLength proves escape sequences aren't counted
// toward the visible-column budget: 5 visible columns plus the color codes
// necessarily exceeds 5 raw bytes.
func TestTruncateVisibleColorByteLength(t *testing.T) {
	got := truncateVisible(cyan("HELLO WORLD"), 5)
	if len(got) <= 5 {
		t.Errorf("expected raw byte length > 5 (escape codes present), got %d bytes: %q", len(got), got)
	}
}
