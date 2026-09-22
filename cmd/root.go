package cmd

import (
	"fmt"
	"os"

	"github.com/AgusRdz/nvy/internal/updater"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "nvy",
	Short:   "Environment variable manager",
	Long:    "nvy manages environment variables at global (user) and local (project) scope.",
	Version: "dev",
}

// updateLifecycleSkip lists commands whose invocation must not trigger
// update machinery: update/auto-update own it explicitly, __bg-update IS the
// background worker (would otherwise recurse), and version must stay fast
// and side-effect-free.
var updateLifecycleSkip = map[string]bool{
	"update":      true,
	"auto-update": true,
	"__bg-update": true,
	"version":     true,
}

// runUpdateLifecycle applies any pending background-staged update, prints a
// nag if one is available, and kicks off a throttled background check — all
// best-effort and never fatal to the running command.
func runUpdateLifecycle(cmd *cobra.Command) {
	if updateLifecycleSkip[cmd.Name()] {
		return
	}
	if updater.IsDev(rootCmd.Version) {
		return
	}
	updater.ApplyPendingUpdate(rootCmd.Version)
	updater.NotifyIfUpdateAvailable(rootCmd.Version)
	updater.BackgroundCheck(rootCmd.Version)
}

// SetVersion sets the version reported by `nvy --version` and `nvy version`.
// Must be called before Execute.
func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pathCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(autoUpdateCmd)
	rootCmd.AddCommand(bgUpdateCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(hideCmd)
	rootCmd.AddCommand(unhideCmd)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		runUpdateLifecycle(cmd)
	}
}
