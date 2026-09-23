//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type windowsPlatform struct{}

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

const (
	hwndBroadcast   = 0xFFFF
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
)

// envSubkey is the HKCU subkey nvy reads and writes for global vars and PATH.
// A var, not a const, so tests can point it at a throwaway key instead of the
// user's real `Environment` — the registry writers must never touch a
// developer's live environment when `go test` runs on Windows.
var envSubkey = `Environment`

// broadcastEnvChange tells running processes the environment block changed, so
// newly spawned processes and GUI apps that listen for WM_SETTINGCHANGE pick up
// registry-persisted global/PATH vars without a re-login. Best-effort: any
// failure is ignored — the registry write already succeeded, and this only
// affects how fast the change propagates.
func broadcastEnvChange() {
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeout.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(smtoAbortIfHung),
		5000, // 5s timeout per window
		uintptr(unsafe.Pointer(&result)),
	)
}

var current Platform = &windowsPlatform{}

func Get() Platform { return current }

func (p *windowsPlatform) ApplyGlobalVar(key, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envSubkey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue(key, value); err != nil {
		return err
	}
	broadcastEnvChange()
	return nil
}

func (p *windowsPlatform) RemoveGlobalVar(key string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envSubkey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	err = k.DeleteValue(key)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	broadcastEnvChange()
	return nil
}

// ExternalVars enumerates every value under HKCU\Environment except Path,
// which the path command owns. A value may be REG_SZ or REG_EXPAND_SZ;
// GetStringValue handles both. Names of any other type are skipped.
func (p *windowsPlatform) ExternalVars() (map[string]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, envSubkey, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("read registry value names: %w", err)
	}

	vars := make(map[string]string, len(names))
	for _, name := range names {
		if strings.EqualFold(name, "Path") {
			continue
		}
		val, _, err := k.GetStringValue(name)
		if err != nil {
			continue
		}
		vars[name] = val
	}
	return vars, nil
}

func (p *windowsPlatform) GetPath() ([]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, envSubkey, registry.QUERY_VALUE)
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
	return splitPathValue(val), nil
}

// splitPathValue splits a raw PATH registry value on ';', trimming whitespace
// and dropping empty segments.
func splitPathValue(val string) []string {
	var entries []string
	for _, e := range strings.Split(val, ";") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// addPathEntry appends entry unless a case-insensitive match already exists
// (Windows paths are case-insensitive), in which case it errors.
func addPathEntry(entries []string, entry string) ([]string, error) {
	for _, e := range entries {
		if strings.EqualFold(e, entry) {
			return nil, fmt.Errorf("%s is already in PATH", entry)
		}
	}
	return append(entries, entry), nil
}

// removePathEntry drops every case-insensitive match of entry, erroring if none
// was present.
func removePathEntry(entries []string, entry string) ([]string, error) {
	filtered := make([]string, 0, len(entries))
	found := false
	for _, e := range entries {
		if strings.EqualFold(e, entry) {
			found = true
		} else {
			filtered = append(filtered, e)
		}
	}
	if !found {
		return nil, fmt.Errorf("%s not found in PATH", entry)
	}
	return filtered, nil
}

func (p *windowsPlatform) AddToPath(entry string) error {
	entries, err := p.GetPath()
	if err != nil {
		return err
	}
	updated, err := addPathEntry(entries, entry)
	if err != nil {
		return err
	}
	return p.writePath(updated)
}

func (p *windowsPlatform) RemoveFromPath(entry string) error {
	entries, err := p.GetPath()
	if err != nil {
		return err
	}
	updated, err := removePathEntry(entries, entry)
	if err != nil {
		return err
	}
	return p.writePath(updated)
}

func (p *windowsPlatform) writePath(entries []string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envSubkey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()
	if err := k.SetExpandStringValue("Path", strings.Join(entries, ";")); err != nil {
		return err
	}
	broadcastEnvChange()
	return nil
}

const nvyHookMarker = "# nvy hook — do not edit"
const nvyHookEndMarker = "# nvy hook end"

func (p *windowsPlatform) ShellHookScript() string {
	return nvyHookMarker + `
if (-not (Test-Path Function:\_nvy_original_prompt)) {
    if (Test-Path Function:\prompt) {
        Copy-Item Function:\prompt Function:\_nvy_original_prompt
    } else {
        function _nvy_original_prompt { "PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) " }
    }
}
function prompt {
    $nvyExports = & nvy export powershell 2>$null | Out-String
    if ($nvyExports.Trim().Length -gt 0) {
        Invoke-Expression $nvyExports
    }
    _nvy_original_prompt
}
` + nvyHookEndMarker + "\n"
}

func (p *windowsPlatform) ShellConfigPath() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		home, _ := os.UserHomeDir()
		profile = home
	}
	return filepath.Join(profile, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
}

// RegisterBackgroundTask registers TN "nvy-check" to run "nvy check" every 4
// hours. It uses basic schtasks, which works WITHOUT elevation — the richer
// ScheduledTasks CIM cmdlets and XML-with-logon-triggers all require admin
// (Access Denied otherwise). An every-4h cadence means a check runs at least
// that often whenever the machine is on; logon/catch-up triggers are a
// Unix-only extra (they'd need admin here).
func (p *windowsPlatform) RegisterBackgroundTask(binaryPath string) error {
	cmd := exec.Command("schtasks", "/Create",
		"/TN", "nvy-check",
		"/TR", `"`+binaryPath+`" check`,
		"/SC", "HOURLY",
		"/MO", "4",
		"/F",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvy: register scheduled task: %w — %s", err, string(out))
	}
	return nil
}

// BackgroundTaskInstalled reports whether the nvy-check scheduled task exists.
func (p *windowsPlatform) BackgroundTaskInstalled() (bool, error) {
	out, err := exec.Command("schtasks", "/Query", "/TN", "nvy-check").CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// schtasks exits non-zero when the task doesn't exist — not an error.
			return false, nil
		}
		return false, fmt.Errorf("schtasks query: %w — %s", err, string(out))
	}
	return true, nil
}

// RemoveBackgroundTask deletes the nvy-check scheduled task, if present.
func (p *windowsPlatform) RemoveBackgroundTask() error {
	out, err := exec.Command("schtasks", "/Delete", "/TN", "nvy-check", "/F").CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// task didn't exist — nothing to remove.
			return nil
		}
		return fmt.Errorf("schtasks delete: %w — %s", err, string(out))
	}
	return nil
}

// Copy copies text to the Windows clipboard via clip.exe.
func (p *windowsPlatform) Copy(text string) error {
	cmd := exec.Command("clip.exe")
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nvy: clip.exe: %w — %s", err, string(out))
	}
	return nil
}
