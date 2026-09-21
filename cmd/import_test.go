package cmd

import (
	"reflect"
	"testing"
)

func TestResolveImports(t *testing.T) {
	managed := map[string]string{
		"MANAGED_KEY": "already-here",
	}
	external := map[string]string{
		"EXT_ONE": "one",
		"EXT_TWO": "two",
	}

	tests := []struct {
		name               string
		keys               []string
		all                bool
		wantToImport       []string
		wantAlreadyManaged []string
		wantNotFound       []string
	}{
		{
			name:         "all imports every external key, sorted",
			all:          true,
			wantToImport: []string{"EXT_ONE", "EXT_TWO"},
		},
		{
			name:         "named key found in external",
			keys:         []string{"EXT_ONE"},
			wantToImport: []string{"EXT_ONE"},
		},
		{
			name:               "named key already managed",
			keys:               []string{"MANAGED_KEY"},
			wantAlreadyManaged: []string{"MANAGED_KEY"},
		},
		{
			name:         "named key not found",
			keys:         []string{"NOPE"},
			wantNotFound: []string{"NOPE"},
		},
		{
			name:               "mixed named keys",
			keys:               []string{"EXT_TWO", "MANAGED_KEY", "NOPE"},
			wantToImport:       []string{"EXT_TWO"},
			wantAlreadyManaged: []string{"MANAGED_KEY"},
			wantNotFound:       []string{"NOPE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toImport, alreadyManaged, notFound := resolveImports(managed, external, tt.keys, tt.all)
			if !reflect.DeepEqual(toImport, tt.wantToImport) {
				t.Errorf("toImport = %v, want %v", toImport, tt.wantToImport)
			}
			if !reflect.DeepEqual(alreadyManaged, tt.wantAlreadyManaged) {
				t.Errorf("alreadyManaged = %v, want %v", alreadyManaged, tt.wantAlreadyManaged)
			}
			if !reflect.DeepEqual(notFound, tt.wantNotFound) {
				t.Errorf("notFound = %v, want %v", notFound, tt.wantNotFound)
			}
		})
	}
}
