package store

import "time"

type GlobalEntry struct {
	Value     string     `json:"value"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Note      string     `json:"note,omitempty"`
}

type LocalMeta struct {
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Note      string     `json:"note,omitempty"`
}

type GlobalStore map[string]GlobalEntry
type LocalMetaStore map[string]LocalMeta
