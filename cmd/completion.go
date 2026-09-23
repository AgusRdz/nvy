package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"github.com/spf13/cobra"
)

const (
	nvyCompletionMarker    = "# nvy completion"
	nvyCompletionEndMarker = "# nvy completion end"
)

// completionShell picks the shell to generate completion for: PowerShell on
// Windows, zsh or bash on Unix based on $SHELL (default bash).
func completionShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	if strings.Contains(os.Getenv("SHELL"), "zsh") {
		return "zsh"
	}
	return "bash"
}

// installCompletion (re)generates the completion script for the current
// shell under ~/.nvy — always, so an upgraded binary's completions stay
// current — and wires the profile to source it, idempotently. Reports
// whether the profile block was newly installed (false means it was already
// there). Best-effort: callers should treat a returned error as a warning,
// not a fatal init failure.
func installCompletion() (bool, error) {
	dir, err := nvyDataDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("nvy: create %s: %w", dir, err)
	}

	var sourceLine string

	switch completionShell() {
	case "powershell":
		scriptPath := filepath.Join(dir, "completion.ps1")
		if err := rootCmd.GenPowerShellCompletionFileWithDesc(scriptPath); err != nil {
			return false, fmt.Errorf("nvy: generate powershell completion: %w", err)
		}
		sourceLine = `. "$env:USERPROFILE\.nvy\completion.ps1"`
	case "zsh":
		scriptPath := filepath.Join(dir, "completion.zsh")
		if err := rootCmd.GenZshCompletionFile(scriptPath); err != nil {
			return false, fmt.Errorf("nvy: generate zsh completion: %w", err)
		}
		sourceLine = `source "$HOME/.nvy/completion.zsh"`
	default:
		scriptPath := filepath.Join(dir, "completion.bash")
		if err := rootCmd.GenBashCompletionFileV2(scriptPath, true); err != nil {
			return false, fmt.Errorf("nvy: generate bash completion: %w", err)
		}
		sourceLine = `source "$HOME/.nvy/completion.bash"`
	}

	configPath := platform.Get().ShellConfigPath()
	if configPath == "" {
		return false, fmt.Errorf("nvy: no shell config path for this platform")
	}

	return installMarkerBlock(configPath, nvyCompletionMarker, nvyCompletionEndMarker, sourceLine)
}

// installMarkerBlock appends a startMarker/body/endMarker block to the file
// at path if startMarker isn't already present. Reports whether it installed
// the block (false means it was already there).
func installMarkerBlock(path, startMarker, endMarker, body string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("nvy: read %s: %w", path, err)
	}
	content := string(data)

	if strings.Contains(content, startMarker) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, fmt.Errorf("nvy: create config dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("nvy: open %s: %w", path, err)
	}
	defer f.Close()

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return false, fmt.Errorf("nvy: write %s: %w", path, err)
		}
	}

	block := "\n" + startMarker + "\n" + body + "\n" + endMarker + "\n"
	if _, err := f.WriteString(block); err != nil {
		return false, fmt.Errorf("nvy: write %s: %w", path, err)
	}
	return true, nil
}

// removeCompletionBlock strips the nvy completion block from the profile at
// path, mirroring removeHookBlock's marker semantics.
func removeCompletionBlock(path string) (bool, error) {
	return removeMarkerBlock(path, nvyCompletionMarker, nvyCompletionEndMarker)
}

// completeVarNames returns global + local variable names (deduped) matching
// the toComplete prefix — used as a ValidArgsFunction for commands that take
// a managed variable name as their first arg.
func completeVarNames(toComplete string) []string {
	seen := map[string]bool{}
	var names []string

	if gs, err := store.LoadGlobal(); err == nil {
		for k := range gs {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}

	if dir, err := os.Getwd(); err == nil {
		if locals, err := store.LoadEnv(dir); err == nil {
			for k := range locals {
				if !seen[k] {
					seen[k] = true
					names = append(names, k)
				}
			}
		}
	}

	return filterPrefix(names, toComplete)
}

// completeExternalNames returns external OS env var names not already
// managed by nvy's global store, matching the toComplete prefix — used as a
// ValidArgsFunction for `import`.
func completeExternalNames(toComplete string) []string {
	gs, err := store.LoadGlobal()
	if err != nil {
		gs = store.GlobalStore{}
	}

	ext, err := platform.Get().ExternalVars()
	if err != nil {
		return nil
	}

	var names []string
	for k := range ext {
		if _, managed := gs[k]; !managed {
			names = append(names, k)
		}
	}

	return filterPrefix(names, toComplete)
}

func filterPrefix(names []string, prefix string) []string {
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// completeFirstArgFunc adapts a name-completion function into a cobra
// ValidArgsFunction that only completes the first positional arg.
func completeFirstArgFunc(fn func(toComplete string) []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return fn(toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}
