package gitignore

import (
	"os"
	"path/filepath"
	"strings"
)

var entries = []string{".env", ".env.nvy"}

// Ensure makes sure .env and .env.nvy are in .gitignore.
// Creates .gitignore if it doesn't exist.
func Ensure(dir string) error {
	path := filepath.Join(dir, ".gitignore")

	var existing string
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing = string(data)

	var toAdd []string
	for _, entry := range entries {
		if !containsLine(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// ensure newline before appending if file has content
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	for _, entry := range toAdd {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}
	return nil
}

func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimRight(l, "\r") == line {
			return true
		}
	}
	return false
}
