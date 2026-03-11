//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type windowsPlatform struct{}

var current Platform = &windowsPlatform{}

func Get() Platform { return current }

func (p *windowsPlatform) ApplyGlobalVar(key, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	return k.SetStringValue(key, value)
}

func (p *windowsPlatform) RemoveGlobalVar(key string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	err = k.DeleteValue(key)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func (p *windowsPlatform) GetPath() ([]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	val, _, err := k.GetStringValue("Path")
	if err == registry.ErrNotExist {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read PATH: %w", err)
	}

	var entries []string
	for _, e := range strings.Split(val, ";") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (p *windowsPlatform) AddToPath(entry string) error {
	entries, err := p.GetPath()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.EqualFold(e, entry) {
			return fmt.Errorf("%s is already in PATH", entry)
		}
	}
	entries = append(entries, entry)
	return p.writePath(entries)
}

func (p *windowsPlatform) RemoveFromPath(entry string) error {
	entries, err := p.GetPath()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	found := false
	for _, e := range entries {
		if strings.EqualFold(e, entry) {
			found = true
		} else {
			filtered = append(filtered, e)
		}
	}
	if !found {
		return fmt.Errorf("%s not found in PATH", entry)
	}
	return p.writePath(filtered)
}

func (p *windowsPlatform) writePath(entries []string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	return k.SetExpandStringValue("Path", strings.Join(entries, ";"))
}

const nvyHookMarker = "# nvy hook — do not edit"

func (p *windowsPlatform) ShellHookScript() string {
	return nvyHookMarker + `
function _nvy_hook {
    $nvyFile = Join-Path (Get-Location) ".env"
    if (Test-Path $nvyFile) {
        Get-Content $nvyFile | ForEach-Object {
            if ($_ -match "^\s*([^#][^=]+)=(.*)$") {
                [System.Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
            }
        }
    }
}
function Set-LocationWithHook { Set-Location @args; _nvy_hook }
Set-Alias -Name cd -Value Set-LocationWithHook -Force
_nvy_hook
`
}

func (p *windowsPlatform) ShellConfigPath() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		home, _ := os.UserHomeDir()
		profile = home
	}
	return filepath.Join(profile, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
}

func (p *windowsPlatform) RegisterBackgroundTask(binaryPath string) error {
	// schtasks /Create /TN "nvy-check" /TR "<binary> check" /SC DAILY /ST 09:00 /F
	cmd := exec.Command("schtasks", "/Create",
		"/TN", "nvy-check",
		"/TR", binaryPath+" check",
		"/SC", "DAILY",
		"/ST", "09:00",
		"/F", // force overwrite if exists
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %w — %s", err, string(out))
	}
	return nil
}
