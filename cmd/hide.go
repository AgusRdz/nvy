package cmd

import (
	"fmt"

	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var hideCmd = &cobra.Command{
	Use:   "hide <name>",
	Short: "Hide an entry from the default `list`/`ui` view",
	Args:  cobra.ExactArgs(1),
	RunE:  runHide,
}

var unhideCmd = &cobra.Command{
	Use:   "unhide <name>",
	Short: "Reveal a previously hidden entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnhide,
}

var (
	hideGlobal bool
	hideLocal  bool
	hidePath   bool
)

func init() {
	for _, c := range []*cobra.Command{hideCmd, unhideCmd} {
		c.Flags().BoolVar(&hideGlobal, "global", false, "target a global var (default)")
		c.Flags().BoolVar(&hideLocal, "local", false, "target a local .env var")
		c.Flags().BoolVar(&hidePath, "path", false, "target a PATH entry")
	}
}

func runHide(_ *cobra.Command, args []string) error {
	return toggleHidden(args[0], true)
}

func runUnhide(_ *cobra.Command, args []string) error {
	return toggleHidden(args[0], false)
}

// hideScope resolves the mutually exclusive --global/--local/--path flags to
// a scope name, defaulting to "global".
func hideScope() (string, error) {
	n := 0
	scope := "global"
	if hideGlobal {
		n++
		scope = "global"
	}
	if hideLocal {
		n++
		scope = "local"
	}
	if hidePath {
		n++
		scope = "path"
	}
	if n > 1 {
		return "", fmt.Errorf("nvy: --global, --local, and --path are mutually exclusive")
	}
	return scope, nil
}

func toggleHidden(name string, hidden bool) error {
	scope, err := hideScope()
	if err != nil {
		return err
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	verb := "hidden"
	if !hidden {
		verb = "unhidden"
	}

	switch scope {
	case "global":
		cfg.SetHiddenGlobal(name, hidden)
	case "local":
		cfg.SetHiddenLocal(name, hidden)
	case "path":
		cfg.SetHiddenPath(name, hidden)
	}

	if err := store.SaveConfig(cfg); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}

	fmt.Printf("%s %s (%s)\n", verb, name, scope)
	return nil
}
