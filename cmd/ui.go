package cmd

import (
	"fmt"

	"github.com/AgusRdz/nvy/internal/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Open the interactive UI",
	Args:  cobra.NoArgs,
	RunE:  runUI,
}

func runUI(_ *cobra.Command, _ []string) error {
	enableVT()
	if err := tui.Run(); err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	return nil
}
