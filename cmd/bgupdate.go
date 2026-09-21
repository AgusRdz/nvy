package cmd

import (
	"github.com/AgusRdz/nvy/internal/updater"
	"github.com/spf13/cobra"
)

// bgUpdateCmd is the detached worker spawned by updater.BackgroundCheck. It
// is hidden from help/usage — it's an internal implementation detail, not a
// user-facing command.
var bgUpdateCmd = &cobra.Command{
	Use:    "__bg-update <version>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		updater.RunBackgroundUpdate(args[0])
		return nil
	},
}
