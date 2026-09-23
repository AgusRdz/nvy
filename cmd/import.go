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

var importCmd = &cobra.Command{
	Use:   "import [KEY...]",
	Short: "Adopt external OS env vars into nvy's global store",
	RunE:  runImport,
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeExternalNames(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
}

var importAll bool

func init() {
	importCmd.Flags().BoolVar(&importAll, "all", false, "import all external vars")
}

func runImport(cmd *cobra.Command, args []string) error {
	if importAll && len(args) > 0 {
		return fmt.Errorf("nvy: --all and explicit keys are mutually exclusive")
	}
	if !importAll && len(args) == 0 {
		return fmt.Errorf("nvy: specify KEY... or --all")
	}

	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	extRaw, err := platform.Get().ExternalVars()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	managed := make(map[string]string, len(gs))
	for k, entry := range gs {
		managed[k] = entry.Value
	}
	external := make(map[string]string, len(extRaw))
	for k, v := range extRaw {
		if _, ok := managed[k]; ok {
			continue
		}
		external[k] = v
	}

	toImport, alreadyManaged, notFound := resolveImports(managed, external, args, importAll)

	for _, k := range alreadyManaged {
		fmt.Printf("%s already managed\n", k)
	}
	for _, k := range notFound {
		fmt.Fprintf(os.Stderr, "nvy: %s not found among external vars\n", k)
	}

	if len(toImport) > 0 {
		now := time.Now().UTC()
		for _, k := range toImport {
			gs[k] = store.GlobalEntry{
				Value:     external[k],
				UpdatedAt: now,
			}
		}
		if err := store.SaveGlobal(gs); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
	}

	fmt.Printf("imported %d var(s)\n", len(toImport))

	if !importAll && len(toImport) == 0 {
		return fmt.Errorf("nvy: no vars imported")
	}
	return nil
}

// resolveImports is the pure adoption decision: given the currently-managed vars,
// the external (OS-only, already excludes managed) vars, and either explicit keys
// or --all, it decides what to import, what's already managed, and what's missing.
func resolveImports(managed, external map[string]string, keys []string, all bool) (toImport []string, alreadyManaged []string, notFound []string) {
	if all {
		for k := range external {
			toImport = append(toImport, k)
		}
		sort.Strings(toImport)
		return
	}

	for _, k := range keys {
		if _, ok := managed[k]; ok {
			alreadyManaged = append(alreadyManaged, k)
			continue
		}
		if _, ok := external[k]; ok {
			toImport = append(toImport, k)
			continue
		}
		notFound = append(notFound, k)
	}
	return
}
