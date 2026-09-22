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
	listPath   bool
	listAll    bool
)

func init() {
	listCmd.Flags().BoolVar(&listGlobal, "global", false, "list global vars only")
	listCmd.Flags().BoolVar(&listLocal, "local", false, "list local .env vars only")
	listCmd.Flags().BoolVar(&listPath, "path", false, "list PATH entries only")
	listCmd.Flags().BoolVar(&listAll, "all", false, "include hidden entries")
}

func runList(cmd *cobra.Command, args []string) error {
	noScope := !listGlobal && !listLocal && !listPath
	showGlobal := listGlobal || noScope
	showLocal := listLocal || noScope
	showPath := listPath // PATH is opt-in; a bare `nvy list` stays global + local

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if showGlobal {
		if err := printGlobalList(cfg, listAll); err != nil {
			return err
		}
	}

	if showLocal {
		if err := printLocalList(cfg, listAll); err != nil {
			return err
		}
	}

	if showPath {
		if err := printPathList(cfg, listAll); err != nil {
			return err
		}
	}

	return nil
}

func printGlobalList(cfg store.Config, all bool) error {
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

	hidden := 0
	keys := sortedKeys(gs)
	for _, k := range keys {
		isHidden := cfg.IsHiddenGlobal(k)
		if isHidden {
			hidden++
			if !all {
				continue
			}
		}
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
		if isHidden {
			line += "  (hidden)"
		}
		fmt.Println(line)
	}

	if extErr != nil {
		fmt.Printf("  (external vars unavailable: %v)\n", extErr)
	} else {
		extKeys := make([]string, 0, len(external))
		for k := range external {
			extKeys = append(extKeys, k)
		}
		sort.Strings(extKeys)
		for _, k := range extKeys {
			isHidden := cfg.IsHiddenGlobal(k)
			if isHidden {
				hidden++
				if !all {
					continue
				}
			}
			line := fmt.Sprintf("  ext  %-30s  %s  (external, read-only)", k, maskValue(external[k]))
			if isHidden {
				line += "  (hidden)"
			}
			fmt.Println(line)
		}
	}

	if hidden > 0 && !all {
		fmt.Printf("  (%d hidden — use --all)\n", hidden)
	}

	return nil
}

func printPathList(cfg store.Config, all bool) error {
	entries, err := platform.Get().GetPath()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("path: (empty)")
		return nil
	}

	fmt.Println("PATH")

	hidden := 0
	for _, e := range entries {
		isHidden := cfg.IsHiddenPath(e)
		if isHidden {
			hidden++
			if !all {
				continue
			}
		}
		line := "  " + e
		if isHidden {
			line += "  (hidden)"
		}
		fmt.Println(line)
	}

	if hidden > 0 && !all {
		fmt.Printf("  (%d hidden — use --all)\n", hidden)
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

func printLocalList(cfg store.Config, all bool) error {
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
	hidden := 0
	for _, k := range keys {
		isHidden := cfg.IsHiddenLocal(k)
		if isHidden {
			hidden++
			if !all {
				continue
			}
		}
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
		if isHidden {
			line += "  (hidden)"
		}
		fmt.Println(line)
	}
	if hidden > 0 && !all {
		fmt.Printf("  (%d hidden — use --all)\n", hidden)
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
