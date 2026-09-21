package cmd

import (
	"github.com/AgusRdz/nvy/internal/updater"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update nvy to the latest release",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return updater.Run(rootCmd.Version)
	},
}
