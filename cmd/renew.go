package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AgusRdz/nvy/internal/gitignore"
	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var renewCmd = &cobra.Command{
	Use:               "renew KEY",
	Short:             "Open a variable's renew page, then save the new value",
	Args:              cobra.ExactArgs(1),
	RunE:              runRenew,
	ValidArgsFunction: completeFirstArgFunc(completeRenewNames),
}

var (
	renewGlobal   bool
	renewLocal    bool
	renewOpenOnly bool
)

func init() {
	renewCmd.Flags().BoolVar(&renewGlobal, "global", false, "use the global var only")
	renewCmd.Flags().BoolVar(&renewLocal, "local", false, "use the .env var of the current directory only")
	renewCmd.Flags().BoolVar(&renewOpenOnly, "open-only", false, "open the renew page without prompting for a new value")
}

func runRenew(_ *cobra.Command, args []string) error {
	key := args[0]

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	// local takes precedence over global, mirroring `get`
	var (
		renewURL string
		isLocal  bool
		meta     store.LocalMetaStore
		gs       store.GlobalStore
	)
	if !renewGlobal {
		locals, err := store.LoadEnv(dir)
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		if _, ok := locals[key]; ok {
			meta, err = store.LoadLocalMeta(dir)
			if err != nil {
				return fmt.Errorf("nvy: %w", err)
			}
			renewURL, isLocal = meta[key].RenewURL, true
		}
	}
	if !isLocal {
		if renewLocal {
			return fmt.Errorf("nvy: %s: not found in .env", key)
		}
		gs, err = store.LoadGlobal()
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		entry, ok := gs[key]
		if !ok {
			return fmt.Errorf("nvy: %s: not found", key)
		}
		renewURL = entry.RenewURL
	}

	scope := "global"
	hint := fmt.Sprintf("nvy set %s --renew-url <url>", key)
	if isLocal {
		scope = "local"
		hint = fmt.Sprintf("nvy set %s --renew-url <url> --local", key)
	}
	if renewURL == "" {
		return fmt.Errorf("nvy: %s has no renew URL; set one with: %s", key, hint)
	}
	// Stored values can be hand-edited; never hand an unvalidated string to the OS opener.
	if err := store.ValidateRenewURL(renewURL); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("opening %s\n", renewURL)
	if err := platform.Get().OpenURL(renewURL); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fd := int(os.Stdin.Fd())
	if renewOpenOnly || !term.IsTerminal(fd) {
		return nil
	}

	fmt.Printf("New value for %s (blank to skip): ", key)
	raw, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	value := string(raw)
	if value == "" {
		return nil
	}

	fmt.Print(`New expiry YYYY-MM-DD (blank keeps current, "none" clears): `)
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && input == "" {
		return fmt.Errorf("nvy: %w", err)
	}
	input = strings.TrimSpace(input)

	var (
		expiresChanged bool
		expiresAt      *time.Time
	)
	switch {
	case input == "":
	case strings.EqualFold(input, "none"):
		expiresChanged = true
	default:
		t, err := time.Parse("2006-01-02", input)
		if err != nil {
			return fmt.Errorf("nvy: invalid date %q: expected YYYY-MM-DD", input)
		}
		expiresChanged, expiresAt = true, &t
	}

	if isLocal {
		if err := gitignore.Ensure(dir); err != nil {
			return fmt.Errorf("nvy: update .gitignore: %w", err)
		}
		if err := store.SetLocalVar(dir, key, value); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		m := meta[key]
		if expiresChanged {
			m.ExpiresAt = expiresAt
		}
		m.UpdatedAt = time.Now().UTC()
		meta[key] = m
		if err := store.SaveLocalMeta(dir, meta); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
	} else {
		entry := gs[key]
		entry.Value = value
		if expiresChanged {
			entry.ExpiresAt = expiresAt
		}
		entry.UpdatedAt = time.Now().UTC()
		gs[key] = entry
		if err := store.SaveGlobal(gs); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		if err := platform.Get().ApplyGlobalVar(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "nvy: warning: failed to apply to OS environment: %v\n", err)
		}
	}

	fmt.Printf("renewed %s (%s)\n", key, scope)
	return nil
}
