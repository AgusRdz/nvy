package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// NotifyState maps a var key to the date (2006-01-02) it was last notified.
type NotifyState map[string]string

func notifyStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".nvy", "notify-state.json"), nil
}

// LoadNotifyState reads ~/.nvy/notify-state.json. Returns an empty state if
// the file doesn't exist.
func LoadNotifyState() (NotifyState, error) {
	path, err := notifyStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NotifyState{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read notify state: %w", err)
	}

	var ns NotifyState
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("parse notify state: %w", err)
	}
	return ns, nil
}

// SaveNotifyState writes ns to ~/.nvy/notify-state.json (temp+rename).
func SaveNotifyState(ns NotifyState) error {
	path, err := notifyStatePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create ~/.nvy dir: %w", err)
	}

	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal notify state: %w", err)
	}
	data = append(data, '\n')

	return writeAtomic(path, data, 0600)
}
