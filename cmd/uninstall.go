package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/spf13/cobra"
)

var uninstallKeepData bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove nvy's shell hook, background task, data, and binary",
	Args:  cobra.NoArgs,
	RunE:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallKeepData, "keep-data", false, "keep the ~/.nvy data directory")
}

func runUninstall(_ *cobra.Command, _ []string) error {
	p := platform.Get()

	// 1. Shell hook.
	shellConfigPath := p.ShellConfigPath()
	if shellConfigPath == "" {
		fmt.Println("shell hook: not supported on this platform — skipped")
	} else {
		removed, err := removeHookBlock(shellConfigPath)
		switch {
		case err != nil:
			fmt.Printf("shell hook: could not update %s: %v\n", shellConfigPath, err)
		case removed:
			fmt.Printf("shell hook: removed from %s (backup: %s.nvy.bak)\n", shellConfigPath, shellConfigPath)
		default:
			fmt.Printf("shell hook: not found in %s\n", shellConfigPath)
		}
	}

	// 2. Background task.
	if err := p.RemoveBackgroundTask(); err != nil {
		fmt.Printf("background task: could not remove: %v\n", err)
	} else {
		fmt.Println("background task: removed (or was not registered)")
	}

	// 3. Data directory.
	if uninstallKeepData {
		fmt.Println("data: kept (--keep-data)")
	} else {
		dir, err := nvyDataDir()
		switch {
		case err != nil:
			fmt.Printf("data: could not determine ~/.nvy: %v\n", err)
		case !fileExists(dir):
			fmt.Printf("data: not found (%s)\n", dir)
		default:
			if err := os.RemoveAll(dir); err != nil {
				fmt.Printf("data: could not remove %s: %v\n", dir, err)
			} else {
				fmt.Printf("data: removed %s\n", dir)
			}
		}
	}

	// 4. Binary — last, since we're deleting the thing that's running.
	exe, err := os.Executable()
	switch {
	case err != nil:
		fmt.Printf("binary: could not determine location: %v\n", err)
	default:
		if err := os.Remove(exe); err != nil {
			fmt.Printf("binary: could not remove %s (%v) — delete it manually\n", exe, err)
		} else {
			fmt.Printf("binary: removed %s\n", exe)
		}
	}

	fmt.Println("nvy: uninstalled")
	return nil
}

// removeHookBlock strips the nvy hook block from the file at path: from the
// start marker line through the end marker line (inclusive), plus a leading
// blank line if present. If no end marker is found after the start marker
// (hooks installed before the end marker existed), it strips through
// end-of-file, since the hook is always appended last. Reports whether a
// block was found and removed.
func removeHookBlock(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("nvy: read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	startIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "# nvy hook") && !strings.Contains(l, "# nvy hook end") {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return false, nil
	}

	endIdx := len(lines) - 1
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], "# nvy hook end") {
			endIdx = i
			break
		}
	}

	removeFrom := startIdx
	if removeFrom > 0 && strings.TrimSpace(lines[removeFrom-1]) == "" {
		removeFrom--
	}

	remaining := append([]string{}, lines[:removeFrom]...)
	if endIdx+1 < len(lines) {
		remaining = append(remaining, lines[endIdx+1:]...)
	}
	result := strings.Join(remaining, "\n")

	// Back up the original profile before rewriting — the no-end-marker fallback
	// strips to end-of-file, so a recoverable copy guards against losing any
	// content a user added after the hook block.
	if err := os.WriteFile(path+".nvy.bak", data, 0644); err != nil {
		return false, fmt.Errorf("nvy: back up %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return false, fmt.Errorf("nvy: write %s: %w", path, err)
	}
	return true, nil
}

// nvyDataDir returns ~/.nvy — the directory store uses for global.json and config.json.
func nvyDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("nvy: %w", err)
	}
	return filepath.Join(home, ".nvy"), nil
}
