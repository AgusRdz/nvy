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

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check nvy's install health",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	issues := 0
	p := platform.Get()

	// 1. Binary location on PATH.
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("[!] could not determine binary location: %v\n", err)
		issues++
	} else {
		dir := filepath.Dir(exe)
		if onPath(dir) {
			fmt.Printf("[ok] binary is on PATH (%s)\n", exe)
		} else {
			fmt.Printf("[!] binary directory is not on PATH: %s\n", dir)
			fmt.Println("    fix: add it to PATH (e.g. `nvy path add " + dir + "`, then restart your shell)")
			issues++
		}
	}

	// 2. Shell hook installed.
	shellConfigPath := p.ShellConfigPath()
	if shellConfigPath == "" {
		fmt.Println("[!] shell hook not supported on this platform")
		issues++
	} else {
		data, err := os.ReadFile(shellConfigPath)
		switch {
		case os.IsNotExist(err):
			fmt.Printf("[!] shell hook not installed (%s not found)\n", shellConfigPath)
			fmt.Println("    fix: nvy init")
			issues++
		case err != nil:
			fmt.Printf("[!] could not read shell config %s: %v\n", shellConfigPath, err)
			issues++
		case !strings.Contains(string(data), "# nvy hook"):
			fmt.Printf("[!] shell hook not installed in %s\n", shellConfigPath)
			fmt.Println("    fix: nvy init")
			issues++
		default:
			fmt.Printf("[ok] shell hook installed (%s)\n", shellConfigPath)
		}
	}

	// 3. Background expiration task registered.
	installed, err := p.BackgroundTaskInstalled()
	switch {
	case err != nil:
		fmt.Printf("[!] could not check background task: %v\n", err)
		issues++
	case !installed:
		fmt.Println("[!] background expiration check task not registered")
		fmt.Println("    fix: nvy init")
		issues++
	default:
		fmt.Println("[ok] background expiration check task registered")
	}

	// 4. Global store valid.
	gs, err := store.LoadGlobal()
	if err != nil {
		fmt.Printf("[!] global store is invalid: %v\n", err)
		issues++
	} else {
		fmt.Printf("[ok] global store valid (%d var(s))\n", len(gs))
	}

	// 5. .gitignore protection for local env files in the current directory.
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[!] could not determine current directory: %v\n", err)
		issues++
	} else {
		envExists := fileExists(filepath.Join(dir, ".env"))
		sidecarExists := fileExists(filepath.Join(dir, ".env.nvy"))
		if !envExists && !sidecarExists {
			fmt.Println("[ok] no local .env file in current directory")
		} else {
			protected, err := gitignoreProtects(dir)
			switch {
			case err != nil:
				fmt.Printf("[!] could not check .gitignore: %v\n", err)
				issues++
			case !protected:
				fmt.Println("[!] .env/.env.nvy exist but are not protected by .gitignore")
				fmt.Println("    fix: any --local write re-applies protection, or add them to .gitignore manually")
				issues++
			default:
				fmt.Println("[ok] .env/.env.nvy are protected by .gitignore")
			}
		}
	}

	if issues == 0 {
		fmt.Println("\nall good")
	} else {
		fmt.Printf("\n%d issue(s) found\n", issues)
	}
	return nil
}

// onPath reports whether dir appears in the PATH environment variable.
func onPath(dir string) bool {
	pathEnv := os.Getenv("PATH")
	for _, entry := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if entry == "" {
			continue
		}
		if samePath(entry, dir) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	ca, errA := filepath.Abs(a)
	cb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// gitignoreProtects reports whether both .env and .env.nvy are listed in
// dir's .gitignore. Read-only — unlike internal/gitignore.Ensure, it never
// writes, since doctor only reports issues.
func gitignoreProtects(dir string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	for _, entry := range []string{".env", ".env.nvy"} {
		found := false
		for _, l := range lines {
			if strings.TrimRight(l, "\r") == entry {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}
