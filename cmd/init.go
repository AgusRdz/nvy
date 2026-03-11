package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/spf13/cobra"
)



var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure shell hook",
	Long:  "Inject the nvy shell hook into your shell config so local .env files are loaded automatically on cd.",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	p := platform.Get()
	hookScript := p.ShellHookScript()
	configPath := p.ShellConfigPath()

	if hookScript == "" || configPath == "" {
		fmt.Println("nvy: shell hook not supported on this platform — add vars manually or use nvy set.")
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("nvy: read %s: %w", configPath, err)
	}
	content := string(data)

	if strings.Contains(content, "# nvy hook") {
		fmt.Printf("nvy: hook already installed in %s\n", configPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("nvy: create config dir: %w", err)
	}

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("nvy: open %s: %w", configPath, err)
	}
	defer f.Close()

	// ensure blank line separator
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("nvy: write %s: %w", configPath, err)
		}
	}
	if _, err := f.WriteString("\n" + hookScript); err != nil {
		return fmt.Errorf("nvy: write %s: %w", configPath, err)
	}

	fmt.Printf("nvy: hook installed in %s\n", configPath)
	fmt.Printf("     restart your shell or run: source %s\n", configPath)

	// register daily background check task
	binary, err := os.Executable()
	if err == nil {
		if err := p.RegisterBackgroundTask(binary); err != nil {
			fmt.Fprintf(os.Stderr, "nvy: warning: could not register background task: %v\n", err)
		} else {
			fmt.Println("nvy: daily expiration check scheduled")
		}
	}

	return nil
}
