//go:build !windows

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type unixPlatform struct{}

var current Platform = &unixPlatform{}

func Get() Platform { return current }

// ApplyGlobalVar is a no-op on Unix — the shell hook reads global.json.
func (p *unixPlatform) ApplyGlobalVar(key, value string) error { return nil }

// RemoveGlobalVar is a no-op on Unix — the shell hook reads global.json.
func (p *unixPlatform) RemoveGlobalVar(key string) error { return nil }

// externalVarsDenylist holds noise vars to exclude from ExternalVars: shell/session
// plumbing that isn't a meaningful user-set env var. Tunable.
var externalVarsDenylist = map[string]bool{
	"PATH": true, "HOME": true, "PWD": true, "OLDPWD": true, "SHELL": true,
	"SHLVL": true, "TERM": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"USER": true, "LOGNAME": true, "HOSTNAME": true, "LANG": true, "LANGUAGE": true,
	"DISPLAY": true, "COLORTERM": true, "PS1": true, "_": true,
}

// externalVarsDenylistPrefixes holds name prefixes to exclude from ExternalVars.
var externalVarsDenylistPrefixes = []string{
	"LC_", "SSH_", "XDG_", "DBUS_", "GPG_", "XAUTH", "GNOME_", "KDE_",
}

// filterExternalUnix splits an os.Environ()-shaped slice into a map, excluding
// the noise denylist. Pure so it's testable without touching the real environment.
func filterExternalUnix(environ []string) map[string]string {
	vars := make(map[string]string, len(environ))
	for _, kv := range environ {
		idx := strings.Index(kv, "=")
		if idx <= 0 {
			continue
		}
		key := kv[:idx]
		if externalVarsDenylist[key] {
			continue
		}
		denied := false
		for _, prefix := range externalVarsDenylistPrefixes {
			if strings.HasPrefix(key, prefix) {
				denied = true
				break
			}
		}
		if denied {
			continue
		}
		vars[key] = kv[idx+1:]
	}
	return vars
}

// ExternalVars reads os.Environ(), minus the noise denylist.
func (p *unixPlatform) ExternalVars() (map[string]string, error) {
	return filterExternalUnix(os.Environ()), nil
}

func (p *unixPlatform) GetPath() ([]string, error) {
	val := os.Getenv("PATH")
	var entries []string
	for _, e := range strings.Split(val, ":") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (p *unixPlatform) AddToPath(entry string) error {
	configPath := p.ShellConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	content := string(data)

	line := `export PATH="$PATH:` + entry + `"  # nvy-path`
	if strings.Contains(content, line) {
		return fmt.Errorf("%s is already in PATH", entry)
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(content), 0644)
}

func (p *unixPlatform) RemoveFromPath(entry string) error {
	configPath := p.ShellConfigPath()

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s not found in PATH", entry)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	line := `export PATH="$PATH:` + entry + `"  # nvy-path`
	var kept []string
	found := false
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimRight(l, "\r") == line {
			found = true
		} else {
			kept = append(kept, l)
		}
	}
	if !found {
		return fmt.Errorf("%s not found in PATH", entry)
	}

	// trim trailing empty line added by split, then re-add newline
	for len(kept) > 0 && kept[len(kept)-1] == "" {
		kept = kept[:len(kept)-1]
	}
	result := strings.Join(kept, "\n") + "\n"
	return os.WriteFile(configPath, []byte(result), 0644)
}

const nvyHookMarker = "# nvy hook — do not edit"
const nvyHookEndMarker = "# nvy hook end"

func (p *unixPlatform) ShellHookScript() string {
	return nvyHookMarker + `
_nvy_hook() {
    if [ -f ".env" ]; then
        set -a
        source .env
        set +a
    fi
}
cd() { builtin cd "$@" && _nvy_hook; }
_nvy_hook
` + nvyHookEndMarker + "\n"
}

func (p *unixPlatform) ShellConfigPath() string {
	shell := os.Getenv("SHELL")
	home, _ := os.UserHomeDir()
	if strings.Contains(shell, "zsh") {
		return filepath.Join(home, ".zshrc")
	}
	return filepath.Join(home, ".bashrc")
}

func (p *unixPlatform) RegisterBackgroundTask(binaryPath string) error {
	if runtime.GOOS == "darwin" {
		return registerLaunchd(binaryPath)
	}
	return registerCron(binaryPath)
}

func registerLaunchd(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plistPath := filepath.Join(dir, "run.nvy.check.plist")
	// StartInterval fires every 4h; RunAtLoad covers at-logon/load. launchd
	// runs a missed StartInterval job on wake, giving catch-up for free.
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>run.nvy.check</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + binaryPath + `</string>
        <string>check</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>StartInterval</key><integer>14400</integer>
</dict>
</plist>
`
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// unload first (ignore error — may not be loaded yet)
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w — %s", err, string(out))
	}
	return nil
}

// registerCron installs two tagged lines: @reboot for at-logon/boot, and an
// every-4h line for the repeat. cron has no native run-if-missed, so
// @reboot + every-4h is the closest approximation to catch-up. Re-running
// this replaces any existing "# nvy-check" lines, making it idempotent.
func registerCron(binaryPath string) error {
	out, _ := exec.Command("crontab", "-l").Output()
	existing := string(out)

	var kept []string
	for _, l := range strings.Split(existing, "\n") {
		if l == "" || strings.Contains(l, "# nvy-check") {
			continue
		}
		kept = append(kept, l)
	}

	kept = append(kept,
		"@reboot "+binaryPath+" check  # nvy-check",
		"0 */4 * * * "+binaryPath+" check  # nvy-check",
	)

	result := strings.Join(kept, "\n") + "\n"
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = bytes.NewBufferString(result)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab: %w — %s", err, string(out))
	}
	return nil
}

// BackgroundTaskInstalled reports whether the daily nvy-check task is registered.
func (p *unixPlatform) BackgroundTaskInstalled() (bool, error) {
	if runtime.GOOS == "darwin" {
		return launchdInstalled()
	}
	return cronInstalled()
}

func launchdInstalled() (bool, error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false, fmt.Errorf("launchctl list: %w", err)
	}
	return strings.Contains(string(out), "run.nvy.check"), nil
}

func cronInstalled() (bool, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// no crontab for this user (or crontab unavailable) — treat as not installed.
		return false, nil
	}
	return strings.Contains(string(out), "# nvy-check"), nil
}

// RemoveBackgroundTask unregisters the daily nvy-check task, if present.
func (p *unixPlatform) RemoveBackgroundTask() error {
	if runtime.GOOS == "darwin" {
		return removeLaunchd()
	}
	return removeCron()
}

func removeLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "run.nvy.check.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func removeCron() error {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// no crontab for this user — nothing to remove.
		return nil
	}

	var kept []string
	found := false
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "# nvy-check") {
			found = true
			continue
		}
		kept = append(kept, l)
	}
	if !found {
		return nil
	}

	for len(kept) > 0 && kept[len(kept)-1] == "" {
		kept = kept[:len(kept)-1]
	}

	if len(kept) == 0 {
		if err := exec.Command("crontab", "-r").Run(); err != nil {
			return nil // best-effort: crontab may already be empty
		}
		return nil
	}

	result := strings.Join(kept, "\n") + "\n"
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = bytes.NewBufferString(result)
	out2, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab: %w — %s", err, string(out2))
	}
	return nil
}

// Copy copies text to the clipboard: pbcopy on darwin, wl-copy or xclip on
// Linux (whichever is found on PATH first).
func (p *unixPlatform) Copy(text string) error {
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "darwin":
		cmd = exec.Command("pbcopy")
	default:
		if path, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command(path)
		} else if path, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command(path, "-selection", "clipboard")
		} else {
			return fmt.Errorf("nvy: no clipboard tool found (install xclip or wl-clipboard)")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nvy: %s: %w — %s", cmd.Path, err, string(out))
	}
	return nil
}
