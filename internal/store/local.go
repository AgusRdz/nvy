package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type envLine struct {
	raw string // original line (blank/comment lines)
	key string // non-empty for KEY=VALUE lines
	val string
}

func parseEnvLines(data string) []envLine {
	var lines []envLine
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			lines = append(lines, envLine{raw: line})
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			lines = append(lines, envLine{
				raw: line,
				key: line[:idx],
				val: line[idx+1:],
			})
		} else {
			lines = append(lines, envLine{raw: line})
		}
	}
	// trim trailing empty line added by split
	if len(lines) > 0 && lines[len(lines)-1].raw == "" && lines[len(lines)-1].key == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func renderEnvLines(lines []envLine) string {
	var sb strings.Builder
	for _, l := range lines {
		if l.key != "" {
			sb.WriteString(l.key + "=" + l.val + "\n")
		} else {
			sb.WriteString(l.raw + "\n")
		}
	}
	return sb.String()
}

// LoadEnv reads .env from dir. Returns empty map if file doesn't exist.
func LoadEnv(dir string) (map[string]string, error) {
	path := filepath.Join(dir, ".env")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}

	vars := map[string]string{}
	for _, l := range parseEnvLines(string(data)) {
		if l.key != "" {
			vars[l.key] = l.val
		}
	}
	return vars, nil
}

// SetLocalVar updates or adds KEY=value in .env (in-place, preserving order).
func SetLocalVar(dir, key, value string) error {
	path := filepath.Join(dir, ".env")

	var lines []envLine
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .env: %w", err)
	}
	if err == nil {
		lines = parseEnvLines(string(data))
	}

	found := false
	for i, l := range lines {
		if l.key == key {
			lines[i].val = value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, envLine{key: key, val: value})
	}

	return writeAtomic(path, []byte(renderEnvLines(lines)), 0600)
}

// RemoveLocalVar removes a key from .env.
func RemoveLocalVar(dir, key string) error {
	path := filepath.Join(dir, ".env")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read .env: %w", err)
	}

	lines := parseEnvLines(string(data))
	filtered := lines[:0]
	for _, l := range lines {
		if l.key != key {
			filtered = append(filtered, l)
		}
	}

	return writeAtomic(path, []byte(renderEnvLines(filtered)), 0600)
}

// LoadLocalMeta reads .env.nvy from dir. Returns empty store if file doesn't exist.
func LoadLocalMeta(dir string) (LocalMetaStore, error) {
	path := filepath.Join(dir, ".env.nvy")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return LocalMetaStore{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .env.nvy: %w", err)
	}

	var meta LocalMetaStore
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse .env.nvy: %w", err)
	}
	return meta, nil
}

// SaveLocalMeta writes .env.nvy. Deletes the file if meta is empty.
func SaveLocalMeta(dir string, meta LocalMetaStore) error {
	path := filepath.Join(dir, ".env.nvy")

	if len(meta) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .env.nvy: %w", err)
	}
	data = append(data, '\n')
	return writeAtomic(path, data, 0600)
}
