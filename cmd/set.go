package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AgusRdz/nvy/internal/gitignore"
	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set KEY=VALUE",
	Short: "Set an environment variable",
	Args:  cobra.ExactArgs(1),
	RunE:  runSet,
}

var (
	setGlobal  bool
	setLocal   bool
	setExpires string
	setNote    string
)

func init() {
	setCmd.Flags().BoolVar(&setGlobal, "global", false, "set in global scope (default)")
	setCmd.Flags().BoolVar(&setLocal, "local", false, "set in .env of current directory")
	setCmd.Flags().StringVar(&setExpires, "expires", "", "expiration date (YYYY-MM-DD)")
	setCmd.Flags().StringVar(&setNote, "note", "", "note or description")
}

func runSet(cmd *cobra.Command, args []string) error {
	kv := args[0]
	idx := strings.Index(kv, "=")
	if idx <= 0 {
		return fmt.Errorf("invalid format: expected KEY=VALUE")
	}
	key := kv[:idx]
	value := kv[idx+1:]

	var expiresAt *time.Time
	if setExpires != "" {
		t, err := time.Parse("2006-01-02", setExpires)
		if err != nil {
			return fmt.Errorf("invalid date %q: expected YYYY-MM-DD", setExpires)
		}
		expiresAt = &t
	}

	if setLocal {
		return setLocalVar(key, value, expiresAt)
	}
	return setGlobalVar(key, value, expiresAt)
}

func setGlobalVar(key, value string, expiresAt *time.Time) error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	gs[key] = store.GlobalEntry{
		Value:     value,
		UpdatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
		Note:      setNote,
	}

	if err := store.SaveGlobal(gs); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if err := platform.Get().ApplyGlobalVar(key, value); err != nil {
		fmt.Fprintf(os.Stderr, "nvy: warning: failed to apply to OS environment: %v\n", err)
	}

	fmt.Printf("set %s (global)\n", key)
	return nil
}

func setLocalVar(key, value string, expiresAt *time.Time) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if err := gitignore.Ensure(dir); err != nil {
		return fmt.Errorf("nvy: update .gitignore: %w", err)
	}

	if err := store.SetLocalVar(dir, key, value); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if setExpires != "" || setNote != "" {
		meta, err := store.LoadLocalMeta(dir)
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		meta[key] = store.LocalMeta{
			UpdatedAt: time.Now().UTC(),
			ExpiresAt: expiresAt,
			Note:      setNote,
		}
		if err := store.SaveLocalMeta(dir, meta); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
	}

	fmt.Printf("set %s (local)\n", key)
	return nil
}
