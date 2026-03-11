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
`
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
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key><integer>9</integer>
        <key>Minute</key><integer>0</integer>
    </dict>
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

func registerCron(binaryPath string) error {
	cronLine := "0 9 * * * " + binaryPath + " check  # nvy-check"

	// read existing crontab
	out, _ := exec.Command("crontab", "-l").Output()
	existing := string(out)

	if strings.Contains(existing, "# nvy-check") {
		return nil // already registered
	}

	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	existing += cronLine + "\n"

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = bytes.NewBufferString(existing)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab: %w — %s", err, string(out))
	}
	return nil
}
