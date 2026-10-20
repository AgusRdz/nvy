package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Config struct {
	NotificationLeadDays int      `json:"notification_lead_days"`
	HiddenGlobals        []string `json:"hidden_globals,omitempty"`
	HiddenLocals         []string `json:"hidden_locals,omitempty"`
	HiddenPath           []string `json:"hidden_path,omitempty"`
	CollapsedSections    []string `json:"collapsed_sections,omitempty"`
	NotificationsEnabled *bool    `json:"notifications_enabled,omitempty"`
	CheckIntervalHours   int      `json:"check_interval_hours,omitempty"`
}

// NotificationsOn reports whether notifications are enabled. Nil (unset)
// defaults to on.
func (c *Config) NotificationsOn() bool {
	return c.NotificationsEnabled == nil || *c.NotificationsEnabled
}

// DefaultCheckIntervalHours is the background-check cadence when unset.
const DefaultCheckIntervalHours = 4

// AllowedCheckIntervalHours lists the valid background-check intervals. Each
// divides 24 so cron spacing is even on every platform.
var AllowedCheckIntervalHours = []int{1, 2, 3, 4, 6, 8, 12, 24}

// ValidateCheckIntervalHours returns an error unless n is an allowed interval.
func ValidateCheckIntervalHours(n int) error {
	for _, a := range AllowedCheckIntervalHours {
		if a == n {
			return nil
		}
	}
	return fmt.Errorf("check-interval-hours must be one of 1, 2, 3, 4, 6, 8, 12, 24 (got %d)", n)
}

// CheckInterval returns the effective background-check interval in hours.
// Zero (unset) defaults to DefaultCheckIntervalHours.
func (c *Config) CheckInterval() int {
	// A hand-edited config.json bypasses `nvy config set` validation; an
	// out-of-set value would produce an unevenly spaced or invalid cron entry.
	if ValidateCheckIntervalHours(c.CheckIntervalHours) != nil {
		return DefaultCheckIntervalHours
	}
	return c.CheckIntervalHours
}

// IsHiddenGlobal reports whether key is marked hidden in the global scope.
func (c *Config) IsHiddenGlobal(key string) bool { return isHidden(c.HiddenGlobals, key) }

// IsHiddenLocal reports whether key is marked hidden in the local scope.
func (c *Config) IsHiddenLocal(key string) bool { return isHidden(c.HiddenLocals, key) }

// IsHiddenPath reports whether the PATH entry is marked hidden.
func (c *Config) IsHiddenPath(entry string) bool { return isHidden(c.HiddenPath, entry) }

// SetHiddenGlobal adds or removes key from the hidden-globals list. Idempotent.
func (c *Config) SetHiddenGlobal(key string, hidden bool) {
	c.HiddenGlobals = setHidden(c.HiddenGlobals, key, hidden)
}

// SetHiddenLocal adds or removes key from the hidden-locals list. Idempotent.
func (c *Config) SetHiddenLocal(key string, hidden bool) {
	c.HiddenLocals = setHidden(c.HiddenLocals, key, hidden)
}

// SetHiddenPath adds or removes entry from the hidden-path list. Idempotent.
func (c *Config) SetHiddenPath(entry string, hidden bool) {
	c.HiddenPath = setHidden(c.HiddenPath, entry, hidden)
}

// IsCollapsed reports whether the named section (global/local/path) starts
// collapsed in `nvy ui`.
func (c *Config) IsCollapsed(section string) bool {
	for _, s := range c.CollapsedSections {
		if s == section {
			return true
		}
	}
	return false
}

// isHidden reports whether key is present in the sorted list.
func isHidden(list []string, key string) bool {
	i := sort.SearchStrings(list, key)
	return i < len(list) && list[i] == key
}

// setHidden adds or removes key from the sorted list, keeping it sorted.
// Idempotent: hiding an already-hidden key or unhiding an absent one is a no-op.
func setHidden(list []string, key string, hidden bool) []string {
	i := sort.SearchStrings(list, key)
	found := i < len(list) && list[i] == key

	if hidden {
		if found {
			return list
		}
		list = append(list, "")
		copy(list[i+1:], list[i:])
		list[i] = key
		return list
	}

	if !found {
		return list
	}
	return append(list[:i], list[i+1:]...)
}

func DefaultConfig() Config {
	return Config{NotificationLeadDays: 7}
}

// ConfigPath returns the path to ~/.nvy/config.json.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".nvy", "config.json"), nil
}

// LoadConfig reads ~/.nvy/config.json. Returns defaults if file doesn't exist.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parse config: %w", err)
	}
	if cfg.NotificationLeadDays == 0 {
		cfg.NotificationLeadDays = 7
	}
	return cfg, nil
}

// SaveConfig writes cfg to ~/.nvy/config.json.
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create ~/.nvy dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	return writeAtomic(path, data, 0600)
}
