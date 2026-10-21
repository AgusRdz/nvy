package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
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
	configPaths := p.ShellConfigPaths()

	if hookScript == "" || len(configPaths) == 0 {
		fmt.Println("nvy: shell hook not supported on this platform — add vars manually or use nvy set.")
		return nil
	}

	changed := false
	for _, configPath := range configPaths {
		c, err := installHook(configPath, hookScript)
		if err != nil {
			return err
		}
		changed = changed || c
	}
	if changed {
		fmt.Println(reloadHint(configPaths[0]))
	}

	// Always (re-)register the background check task, even when the hook was
	// already installed — this is what lets an upgraded binary apply a newer
	// schedule. Registration is idempotent (Register-ScheduledTask -Force,
	// cron/launchd replace their tagged entries).
	cfg, cfgErr := store.LoadConfig()
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "nvy: warning: %v (using default check interval)\n", cfgErr)
	}
	binary, err := os.Executable()
	if err == nil {
		if err := p.RegisterBackgroundTask(binary, cfg.CheckInterval()); err != nil {
			fmt.Fprintf(os.Stderr, "nvy: warning: could not register background task: %v\n", err)
		} else {
			fmt.Printf("nvy: expiration check scheduled (every ~%dh)\n", cfg.CheckInterval())
		}
	}

	// Shell completion — best-effort, never fails init.
	installed, err := installCompletion()
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "nvy: warning: could not install shell completion: %v\n", err)
	case installed:
		fmt.Println(completionReloadMsg())
	default:
		fmt.Println("nvy: shell completion already installed")
	}

	return nil
}

// installHook installs or upgrades the hook block in the profile at
// configPath and reports whether the file was written.
func installHook(configPath, hookScript string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("nvy: read %s: %w", configPath, err)
	}
	content := string(data)

	// Replace a hook block from an older nvy (e.g. the Unix hook that sourced
	// .env as shell code) so re-running init upgrades it.
	outdated := strings.Contains(content, "# nvy hook") && !strings.Contains(content, hookScript)
	if outdated {
		if _, err := removeHookBlock(configPath); err != nil {
			return false, err
		}
		data, err = os.ReadFile(configPath)
		if err != nil {
			return false, fmt.Errorf("nvy: read %s: %w", configPath, err)
		}
		content = string(data)
	}

	if strings.Contains(content, "# nvy hook") {
		fmt.Printf("nvy: hook already installed in %s\n", configPath)
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return false, fmt.Errorf("nvy: create config dir: %w", err)
	}

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("nvy: open %s: %w", configPath, err)
	}
	defer f.Close()

	// ensure blank line separator
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return false, fmt.Errorf("nvy: write %s: %w", configPath, err)
		}
	}
	if _, err := f.WriteString("\n" + hookScript); err != nil {
		return false, fmt.Errorf("nvy: write %s: %w", configPath, err)
	}

	if outdated {
		fmt.Printf("nvy: hook updated in %s (previous version backed up to %s.nvy.bak)\n", configPath, configPath)
	} else {
		fmt.Printf("nvy: hook installed in %s\n", configPath)
	}
	return true, nil
}

// reloadHint tells the user how to load the profile into the current shell;
// PowerShell has no `source`, it dot-sources.
func reloadHint(configPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("     restart your shell or run: . \"%s\"", configPath)
	}
	return fmt.Sprintf("     restart your shell or run: source %s", configPath)
}

// completionReloadMsg is the completion-installed line, with a reload hint
// that is valid for the OS's shell.
func completionReloadMsg() string {
	if runtime.GOOS == "windows" {
		return "nvy: shell completion installed (restart PowerShell or run: . $PROFILE)"
	}
	return "nvy: shell completion installed (restart your shell or re-source your profile)"
}
