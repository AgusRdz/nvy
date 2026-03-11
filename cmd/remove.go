package cmd

import (
	"fmt"
	"os"

	"github.com/AgusRdz/nvy/internal/gitignore"
	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove KEY",
	Aliases: []string{"rm"},
	Short:   "Remove an environment variable",
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
}

var (
	removeGlobal bool
	removeLocal  bool
)

func init() {
	removeCmd.Flags().BoolVar(&removeGlobal, "global", false, "remove from global scope (default)")
	removeCmd.Flags().BoolVar(&removeLocal, "local", false, "remove from .env of current directory")
}

func runRemove(cmd *cobra.Command, args []string) error {
	key := args[0]

	if removeLocal {
		return removeLocalVar(key)
	}
	return removeGlobalVar(key)
}

func removeGlobalVar(key string) error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	if _, ok := gs[key]; !ok {
		return fmt.Errorf("nvy: %s: not found", key)
	}

	delete(gs, key)

	if err := store.SaveGlobal(gs); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if err := platform.Get().RemoveGlobalVar(key); err != nil {
		fmt.Fprintf(os.Stderr, "nvy: warning: failed to remove from OS environment: %v\n", err)
	}

	fmt.Printf("removed %s (global)\n", key)
	return nil
}

func removeLocalVar(key string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if err := gitignore.Ensure(dir); err != nil {
		return fmt.Errorf("nvy: update .gitignore: %w", err)
	}

	if err := store.RemoveLocalVar(dir, key); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	// clean up metadata if present
	meta, err := store.LoadLocalMeta(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	if _, ok := meta[key]; ok {
		delete(meta, key)
		if err := store.SaveLocalMeta(dir, meta); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
	}

	fmt.Printf("removed %s (local)\n", key)
	return nil
}
