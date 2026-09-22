package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View nvy configuration",
	Args:  cobra.NoArgs,
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config file in your editor",
	Args:  cobra.NoArgs,
	RunE:  runConfigEdit,
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	path, err := store.ConfigPath()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("config file: %s\n", path)
	fmt.Printf("notification-lead-days = %d\n", cfg.NotificationLeadDays)
	fmt.Printf("collapsed-sections = %s\n", formatCollapsedSections(cfg.CollapsedSections))
	return nil
}

func runConfigSet(_ *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	display := value
	switch key {
	case "notification-lead-days":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("nvy: notification-lead-days must be an integer: %w", err)
		}
		cfg.NotificationLeadDays = n
	case "collapsed-sections":
		sections, err := parseCollapsedSections(value)
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		cfg.CollapsedSections = sections
		display = formatCollapsedSections(sections)
	default:
		return fmt.Errorf("nvy: unknown config key %q", key)
	}

	if err := store.SaveConfig(cfg); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("nvy: set %s = %s\n", key, display)
	return nil
}

// parseCollapsedSections parses the comma-separated collapsed-sections value:
// "" or "none" clears it, otherwise each token must be global/local/path.
// Duplicates are collapsed and the result is returned in canonical order.
func parseCollapsedSections(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "none") {
		return nil, nil
	}

	seen := make(map[string]bool)
	for _, tok := range strings.Split(value, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		switch tok {
		case "global", "local", "path":
			seen[tok] = true
		default:
			return nil, fmt.Errorf("invalid section %q (want global|local|path)", tok)
		}
	}

	var sections []string
	for _, s := range []string{"global", "local", "path"} {
		if seen[s] {
			sections = append(sections, s)
		}
	}
	return sections, nil
}

// formatCollapsedSections renders the collapsed-sections config value for display.
func formatCollapsedSections(sections []string) string {
	if len(sections) == 0 {
		return "(none)"
	}
	return strings.Join(sections, ",")
}

func runConfigEdit(_ *cobra.Command, _ []string) error {
	path, err := store.ConfigPath()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	// make sure there's a file to open, with defaults in place.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		if err := store.SaveConfig(store.DefaultConfig()); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}

	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("nvy: open editor %q: %w", editor, err)
	}
	return nil
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configEditCmd)
}
