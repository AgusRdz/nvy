package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

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
	return nil
}

func runConfigSet(_ *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	switch key {
	case "notification-lead-days":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("nvy: notification-lead-days must be an integer: %w", err)
		}
		cfg.NotificationLeadDays = n
	default:
		return fmt.Errorf("nvy: unknown config key %q", key)
	}

	if err := store.SaveConfig(cfg); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("nvy: set %s = %s\n", key, value)
	return nil
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
