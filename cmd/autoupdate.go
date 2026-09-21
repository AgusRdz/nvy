package cmd

import (
	"fmt"

	"github.com/AgusRdz/nvy/internal/updater"
	"github.com/spf13/cobra"
)

var autoUpdateCmd = &cobra.Command{
	Use:       "auto-update [on|off]",
	Short:     "Show or change automatic background updates",
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"on", "off"},
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			if updater.AutoUpdateEnabled() {
				fmt.Println("auto-update: on")
			} else {
				fmt.Println("auto-update: off")
				fmt.Println("nvy will notify you when updates are available")
				fmt.Println("run 'nvy auto-update on' to enable automatic updates")
			}
			return nil
		}

		switch args[0] {
		case "on":
			if updater.AutoUpdateEnabled() {
				fmt.Println("auto-update is already on")
				return nil
			}
			if err := updater.SetAutoUpdate(true); err != nil {
				return err
			}
			fmt.Println("auto-update enabled — nvy will update itself in the background")
		case "off":
			if !updater.AutoUpdateEnabled() {
				fmt.Println("auto-update is already off")
				return nil
			}
			if err := updater.SetAutoUpdate(false); err != nil {
				return err
			}
			fmt.Println("auto-update disabled — run 'nvy update' to update manually")
		}
		return nil
	},
}
