package cmd

import (
	"fmt"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:   "path",
	Short: "Manage PATH entries",
}

var pathAddCmd = &cobra.Command{
	Use:   "add <entry>",
	Short: "Add an entry to PATH",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if err := platform.Get().AddToPath(args[0]); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		fmt.Printf("added %s to PATH\n", args[0])
		fmt.Println("restart your shell for the change to take effect")
		return nil
	},
}

var pathRemoveCmd = &cobra.Command{
	Use:   "remove <entry>",
	Short: "Remove an entry from PATH",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if err := platform.Get().RemoveFromPath(args[0]); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		fmt.Printf("removed %s from PATH\n", args[0])
		fmt.Println("restart your shell for the change to take effect")
		return nil
	},
}

var pathListCmd = &cobra.Command{
	Use:   "list",
	Short: "List PATH entries",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		entries, err := platform.Get().GetPath()
		if err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		for _, e := range entries {
			fmt.Println(e)
		}
		return nil
	},
}

func init() {
	pathCmd.AddCommand(pathAddCmd)
	pathCmd.AddCommand(pathRemoveCmd)
	pathCmd.AddCommand(pathListCmd)
}
