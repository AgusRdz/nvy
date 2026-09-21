package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	RunE:  runList,
}

var (
	listGlobal bool
	listLocal  bool
)

func init() {
	listCmd.Flags().BoolVar(&listGlobal, "global", false, "list global vars only")
	listCmd.Flags().BoolVar(&listLocal, "local", false, "list local .env vars only")
}

func runList(cmd *cobra.Command, args []string) error {
	showGlobal := listGlobal || (!listGlobal && !listLocal)
	showLocal := listLocal || (!listGlobal && !listLocal)

	if showGlobal {
		if err := printGlobalList(); err != nil {
			return err
		}
	}

	if showLocal {
		if err := printLocalList(); err != nil {
			return err
		}
	}

	return nil
}

func printGlobalList() error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	external, extErr := platform.Get().ExternalVars()
	if extErr == nil {
		for k := range gs {
			delete(external, k)
		}
	}

	if len(gs) == 0 && len(external) == 0 {
		fmt.Println("global: (empty)")
		if extErr != nil {
			fmt.Printf("  (external vars unavailable: %v)\n", extErr)
		}
		return nil
	}

	fmt.Println("GLOBAL")

	keys := sortedKeys(gs)
	for _, k := range keys {
		entry := gs[k]
		line := fmt.Sprintf("  nvy  %-30s", k)
		line += fmt.Sprintf("  updated %s", entry.UpdatedAt.Local().Format("2006-01-02"))
		if entry.ExpiresAt != nil {
			days := int(time.Until(*entry.ExpiresAt).Hours() / 24)
			switch {
			case days < 0:
				line += fmt.Sprintf("  ⚠ expired %d days ago", -days)
			case days <= 7:
				line += fmt.Sprintf("  ⚠ expires in %d days", days)
			default:
				line += fmt.Sprintf("  expires %s", entry.ExpiresAt.Format("2006-01-02"))
			}
		}
		if entry.Note != "" {
			line += fmt.Sprintf("  [%s]", entry.Note)
		}
		fmt.Println(line)
	}

	if extErr != nil {
		fmt.Printf("  (external vars unavailable: %v)\n", extErr)
		return nil
	}

	extKeys := make([]string, 0, len(external))
	for k := range external {
		extKeys = append(extKeys, k)
	}
	sort.Strings(extKeys)
	for _, k := range extKeys {
		fmt.Printf("  ext  %-30s  %s  (external, read-only)\n", k, maskValue(external[k]))
	}

	return nil
}

// maskValue previews an external value without exposing the secret: up to the
// first 4 runes, then a fixed mask that reveals neither the tail nor the length.
func maskValue(v string) string {
	r := []rune(v)
	if len(r) == 0 {
		return "••••"
	}
	n := 4
	if len(r) < n {
		n = len(r)
	}
	return string(r[:n]) + "••••"
}

func printLocalList() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	locals, err := store.LoadEnv(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	meta, err := store.LoadLocalMeta(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if len(locals) == 0 {
		fmt.Println("local: (no .env or empty)")
		return nil
	}

	keys := make([]string, 0, len(locals))
	for k := range locals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("LOCAL (%s/.env)\n", dir)
	for _, k := range keys {
		line := fmt.Sprintf("  %-30s", k)
		if m, ok := meta[k]; ok {
			line += fmt.Sprintf("  updated %s", m.UpdatedAt.Local().Format("2006-01-02"))
			if m.ExpiresAt != nil {
				days := int(time.Until(*m.ExpiresAt).Hours() / 24)
				switch {
				case days < 0:
					line += fmt.Sprintf("  ⚠ expired %d days ago", -days)
				case days <= 7:
					line += fmt.Sprintf("  ⚠ expires in %d days", days)
				default:
					line += fmt.Sprintf("  expires %s", m.ExpiresAt.Format("2006-01-02"))
				}
			}
			if m.Note != "" {
				line += fmt.Sprintf("  [%s]", m.Note)
			}
		}
		fmt.Println(line)
	}
	return nil
}

func sortedKeys(gs store.GlobalStore) []string {
	keys := make([]string, 0, len(gs))
	for k := range gs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
