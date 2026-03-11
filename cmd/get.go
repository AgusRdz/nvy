package cmd

import (
	"fmt"
	"os"

	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get an environment variable value",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

var (
	getGlobal bool
	getLocal  bool
)

func init() {
	getCmd.Flags().BoolVar(&getGlobal, "global", false, "get from global scope only")
	getCmd.Flags().BoolVar(&getLocal, "local", false, "get from .env of current directory only")
}

func runGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	if getLocal {
		return printLocalVar(key)
	}
	if getGlobal {
		return printGlobalVar(key)
	}

	// default: local takes precedence over global
	dir, _ := os.Getwd()
	locals, err := store.LoadEnv(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	if v, ok := locals[key]; ok {
		fmt.Println(v)
		return nil
	}
	return printGlobalVar(key)
}

func printGlobalVar(key string) error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	entry, ok := gs[key]
	if !ok {
		return fmt.Errorf("nvy: %s: not found", key)
	}
	fmt.Println(entry.Value)
	return nil
}

func printLocalVar(key string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	locals, err := store.LoadEnv(dir)
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	v, ok := locals[key]
	if !ok {
		return fmt.Errorf("nvy: %s: not found in .env", key)
	}
	fmt.Println(v)
	return nil
}
