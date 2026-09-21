package cmd

import "testing"

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
