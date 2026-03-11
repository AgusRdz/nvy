package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func globalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".nvy", "global.json"), nil
}

// LoadGlobal reads ~/.nvy/global.json. Returns empty store if file doesn't exist.
func LoadGlobal() (GlobalStore, error) {
	path, err := globalPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return GlobalStore{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read global store: %w", err)
	}

	var gs GlobalStore
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("parse global store: %w", err)
	}
	return gs, nil
}

// SaveGlobal writes GlobalStore to ~/.nvy/global.json (temp+rename).
func SaveGlobal(gs GlobalStore) error {
	path, err := globalPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create ~/.nvy dir: %w", err)
	}

	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal global store: %w", err)
	}
	data = append(data, '\n')

	return writeAtomic(path, data, 0600)
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
