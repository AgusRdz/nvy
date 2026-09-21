package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const checkInterval = 24 * time.Hour

// AutoUpdateEnabled reports whether automatic background updates are turned
// on. Default is off — the flag file must be explicitly created.
func AutoUpdateEnabled() bool {
	p, err := autoUpdateFlagPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// SetAutoUpdate enables or disables automatic background updates.
func SetAutoUpdate(on bool) error {
	p, err := autoUpdateFlagPath()
	if err != nil {
		return fmt.Errorf("nvy: %w", err)
	}
	if on {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			return fmt.Errorf("nvy: %w", err)
		}
		return nil
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("nvy: %w", err)
	}
	return nil
}

// shouldCheck reports whether enough time has passed since the last
// background update check.
func shouldCheck() bool {
	path, err := lastCheckPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return true // never checked
	}
	return time.Since(info.ModTime()) > checkInterval
}

func touchLastCheck() {
	path, err := lastCheckPath()
	if err != nil {
		return
	}
	os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)), 0o600)
}

// BackgroundCheck spawns a detached subprocess to check for updates, at most
// once per 24h. Never blocks and never fails loudly.
func BackgroundCheck(currentVersion string) {
	if IsDev(currentVersion) {
		return
	}
	if !shouldCheck() {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, "__bg-update", currentVersion)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if cmd.Start() == nil {
		touchLastCheck()
	}
}

// RunBackgroundUpdate is the detached worker spawned by BackgroundCheck. It
// checks the latest version, records it for NotifyIfUpdateAvailable, and —
// when auto-update is enabled — downloads and verifies the new binary,
// staging it as a pending update for ApplyPendingUpdate to install on the
// next foreground run.
func RunBackgroundUpdate(currentVersion string) {
	latest, err := LatestVersion()
	if err != nil || !IsNewer(latest, currentVersion) {
		clearUpdateAvailable()
		return
	}

	recordUpdateAvailable(latest)

	if !AutoUpdateEnabled() {
		return
	}

	tmpPath, err := pendingBinaryPath()
	if err != nil {
		return
	}

	binaryName := buildBinaryName()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, binaryName)

	if err := download(url, tmpPath); err != nil {
		os.Remove(tmpPath)
		return
	}

	if err := verifyChecksumAndHash(tmpPath, binaryName, latest); err != nil {
		os.Remove(tmpPath)
		return
	}

	hash, err := hashFile(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return
	}

	pending, err := pendingUpdatePath()
	if err != nil {
		os.Remove(tmpPath)
		return
	}

	content := fmt.Sprintf("%s\n%s\n%s", latest, tmpPath, hash)
	os.WriteFile(pending, []byte(content), 0o600)
}

// ApplyPendingUpdate installs a background-staged update on the next
// foreground run. Silent and best-effort — never disrupts the current
// command.
func ApplyPendingUpdate(currentVersion string) {
	if IsDev(currentVersion) {
		return
	}

	pending, err := pendingUpdatePath()
	if err != nil {
		return
	}

	data, err := os.ReadFile(pending)
	if err != nil {
		return
	}

	if !AutoUpdateEnabled() {
		parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
		os.Remove(pending)
		if len(parts) >= 2 {
			os.Remove(parts[1])
		}
		return
	}

	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(parts) != 3 {
		os.Remove(pending)
		return
	}

	newVersion := parts[0]
	tmpBinary := parts[1]
	expectedHash := parts[2]

	if !IsNewer(newVersion, currentVersion) {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}

	// Guard against path traversal: the pending binary must live inside our
	// own data directory.
	safeDir, err := dataDir()
	if err != nil {
		os.Remove(pending)
		return
	}
	cleanBinary := filepath.Clean(tmpBinary)
	if !strings.HasPrefix(cleanBinary, safeDir+string(filepath.Separator)) {
		os.Remove(pending)
		return
	}
	tmpBinary = cleanBinary

	// Re-hash immediately before install to close the TOCTOU window between
	// download and apply.
	info, err := os.Stat(tmpBinary)
	if err != nil || info.Size() < 1024 {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}
	actualHash, err := hashFile(tmpBinary)
	if err != nil || actualHash != expectedHash {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		os.Remove(pending)
		return
	}

	if err := replaceBinary(tmpBinary, exe); err != nil {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}

	os.Remove(pending)
	clearUpdateAvailable()
	fmt.Fprintf(os.Stderr, "nvy: auto-updated %s -> %s\n", currentVersion, newVersion)
}

// NotifyIfUpdateAvailable prints a hint to stderr when a newer version is
// known and auto-update is off. Silent on all errors.
func NotifyIfUpdateAvailable(currentVersion string) {
	if IsDev(currentVersion) || AutoUpdateEnabled() {
		return
	}
	p, err := updateAvailablePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	latest := strings.TrimSpace(string(data))
	if latest == "" || !IsNewer(latest, currentVersion) {
		return
	}
	fmt.Fprintf(os.Stderr, "nvy: update available %s -> %s (run 'nvy update')\n", currentVersion, latest)
}

func clearUpdateAvailable() {
	p, err := updateAvailablePath()
	if err != nil {
		return
	}
	os.Remove(p)
}

func recordUpdateAvailable(version string) {
	p, err := updateAvailablePath()
	if err != nil {
		return
	}
	os.WriteFile(p, []byte(version), 0o600)
}
