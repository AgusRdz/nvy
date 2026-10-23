package store

import "testing"

func TestValidateRenewURL(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://gitlab.com/-/user_settings/personal_access_tokens", true},
		{"http://localhost:8080/tokens", true},
		{"javascript:alert(1)", false},
		{"file:///etc/passwd", false},
		{"notaurl", false},
		{"https://", false},
		{"http:///path", false},
		{"", false},
		{"ftp://example.com/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := ValidateRenewURL(tt.url)
			if (err == nil) != tt.ok {
				t.Errorf("ValidateRenewURL(%q) err = %v, want ok=%v", tt.url, err, tt.ok)
			}
		})
	}
}
