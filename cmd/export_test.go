package cmd

import "testing"

func TestRenderPowershellExport(t *testing.T) {
	tests := []struct {
		name        string
		desired     map[string]string
		prevApplied []string
		want        string
	}{
		{
			name:        "set only",
			desired:     map[string]string{"FOO": "bar"},
			prevApplied: nil,
			want: "[Environment]::SetEnvironmentVariable('FOO','bar','Process')\n" +
				"$env:NVY_APPLIED='FOO'\n",
		},
		{
			name:        "set and unset",
			desired:     map[string]string{"FOO": "bar"},
			prevApplied: []string{"STALE"},
			want: "[Environment]::SetEnvironmentVariable('FOO','bar','Process')\n" +
				"[Environment]::SetEnvironmentVariable('STALE',$null,'Process')\n" +
				"$env:NVY_APPLIED='FOO'\n",
		},
		{
			name:        "local overrides global",
			desired:     map[string]string{"FOO": "local-wins"},
			prevApplied: []string{"FOO"},
			want: "[Environment]::SetEnvironmentVariable('FOO','local-wins','Process')\n" +
				"$env:NVY_APPLIED='FOO'\n",
		},
		{
			name:        "single quote escaping in value",
			desired:     map[string]string{"FOO": "it's a test"},
			prevApplied: nil,
			want: "[Environment]::SetEnvironmentVariable('FOO','it''s a test','Process')\n" +
				"$env:NVY_APPLIED='FOO'\n",
		},
		{
			name:        "unsafe key is skipped",
			desired:     map[string]string{"FOO": "bar", "BAD-KEY": "x", "1BAD": "y"},
			prevApplied: nil,
			want: "[Environment]::SetEnvironmentVariable('FOO','bar','Process')\n" +
				"$env:NVY_APPLIED='FOO'\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPowershellExport(tt.desired, tt.prevApplied)
			if got != tt.want {
				t.Errorf("renderPowershellExport() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
