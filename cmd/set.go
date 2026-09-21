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
	Use:   "set KEY[=VALUE]",
	Short: "Set an environment variable, or its expiry/note",
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
	setCmd.Flags().StringVar(&setExpires, "expires", "", "expiration date (YYYY-MM-DD); \"none\" or \"\" clears an existing expiry")
	setCmd.Flags().StringVar(&setNote, "note", "", "note or description")
}

// parseSetArg splits args[0] into key/value for the value-setting path. A
// bare KEY (no "=") returns hasValue=false, routing to the metadata-only
// update path.
func parseSetArg(arg string) (key, value string, hasValue bool) {
	if !strings.Contains(arg, "=") {
		return arg, "", false
	}
	idx := strings.Index(arg, "=")
	return arg[:idx], arg[idx+1:], true
}

func runSet(cmd *cobra.Command, args []string) error {
	expiresChanged := cmd.Flags().Changed("expires")
	noteChanged := cmd.Flags().Changed("note")

	var expiresAt *time.Time
	if expiresChanged {
		if setExpires == "" || strings.EqualFold(setExpires, "none") {
			expiresAt = nil
		} else {
			t, err := time.Parse("2006-01-02", setExpires)
			if err != nil {
				return fmt.Errorf("nvy: invalid date %q: expected YYYY-MM-DD", setExpires)
			}
			expiresAt = &t
		}
	}

	key, value, hasValue := parseSetArg(args[0])

	if hasValue {
		if key == "" {
			return fmt.Errorf("nvy: invalid format: expected KEY=VALUE")
		}
		if setLocal {
			return setLocalVar(key, value, expiresAt)
		}
		return setGlobalVar(key, value, expiresAt)
	}

	if !expiresChanged && !noteChanged {
		return fmt.Errorf("nvy: nothing to set; provide a value or --expires/--note")
	}
	if setLocal {
		return setLocalMeta(key, expiresAt, expiresChanged, noteChanged)
	}
	return setGlobalMeta(key, expiresAt, expiresChanged, noteChanged)
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

// setGlobalMeta updates ExpiresAt/Note on an existing managed global var
// without touching its Value. A key that is not yet managed but exists as an
// external OS var is adopted (its current value snapshotted into
// global.json, mirroring `nvy import`) and stamped in the same step.
func setGlobalMeta(key string, expiresAt *time.Time, expiresChanged, noteChanged bool) error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	entry, managed := gs[key]
	if !managed {
		ext, err := platform.Get().ExternalVars()
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		val, found := ext[key]
		if !found {
			return fmt.Errorf("nvy: %s not found; provide a value (nvy set %s=value --expires ...)", key, key)
		}
		entry = store.GlobalEntry{Value: val}
	}

	if expiresChanged {
		entry.ExpiresAt = expiresAt
	}
	if noteChanged {
		entry.Note = setNote
	}
	entry.UpdatedAt = time.Now().UTC()
	gs[key] = entry

	if err := store.SaveGlobal(gs); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("set %s (global)\n", key)
	return nil
}

// setLocalMeta updates ExpiresAt/Note in the .env.nvy sidecar for an
// existing local var without touching its value in .env.
func setLocalMeta(key string, expiresAt *time.Time, expiresChanged, noteChanged bool) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	env, err := store.LoadEnv(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	if _, found := env[key]; !found {
		return fmt.Errorf("nvy: %s not found; provide a value (nvy set %s=value --expires ...)", key, key)
	}

	meta, err := store.LoadLocalMeta(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	m := meta[key]
	if expiresChanged {
		m.ExpiresAt = expiresAt
	}
	if noteChanged {
		m.Note = setNote
	}
	m.UpdatedAt = time.Now().UTC()
	meta[key] = m

	if err := store.SaveLocalMeta(dir, meta); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("set %s (local)\n", key)
	return nil
}
