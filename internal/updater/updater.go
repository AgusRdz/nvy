// Package updater implements nvy's self-update: checking GitHub releases,
// verifying signed checksums, and swapping the running binary.
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const repo = "AgusRdz/nvy"

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

// LatestVersion fetches the tag_name of the latest GitHub release.
func LatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("nvy: could not reach GitHub (check your internet connection or firewall): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("nvy: GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("nvy: failed to parse GitHub API response: %w", err)
	}
	return release.TagName, nil
}

// Run performs a manual "nvy update": check the latest release, and if it is
// newer than currentVersion, download, verify, and swap the running binary.
func Run(currentVersion string) error {
	if IsDev(currentVersion) {
		fmt.Println("nvy: dev build, skipping self-update")
		return nil
	}

	fmt.Println("nvy: checking for updates...")

	latest, err := LatestVersion()
	if err != nil {
		return err
	}

	if !IsNewer(latest, currentVersion) {
		fmt.Printf("nvy is already up to date (%s)\n", currentVersion)
		return nil
	}

	fmt.Printf("nvy: updating %s -> %s\n", currentVersion, latest)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("nvy: failed to find current binary: %w", err)
	}

	binaryName := buildBinaryName()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, binaryName)

	tmpPath := exe + ".tmp"
	if err := download(url, tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("nvy: update failed: %w", err)
	}

	if err := verifyChecksumAndHash(tmpPath, binaryName, latest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("nvy: verification failed: %w", err)
	}

	if err := replaceBinary(tmpPath, exe); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("nvy: failed to replace binary: %w", err)
	}

	clearUpdateAvailable()

	fmt.Printf("nvy: updated to %s\n", latest)
	return nil
}

func buildBinaryName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("nvy-%s-%s%s", goos, goarch, ext)
}

// download fetches url to dest, requiring at least 1024 bytes and a binary
// that matches the current platform's expected magic bytes.
func download(url, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d for %s", resp.StatusCode, url)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("failed to verify downloaded file: %w", err)
	}
	if info.Size() < 1024 {
		return fmt.Errorf("downloaded file too small (%d bytes), release may not exist", info.Size())
	}
	if err := checkBinaryMagic(dest); err != nil {
		return err
	}

	return nil
}

// checkBinaryMagic verifies the file at path starts with the expected magic
// bytes for the current platform (ELF on Linux, Mach-O on macOS, PE on
// Windows).
func checkBinaryMagic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open binary for validation: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	if _, err := io.ReadFull(f, buf); err != nil {
		return fmt.Errorf("binary too small to read magic bytes: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		if buf[0] != 0x7f || buf[1] != 'E' || buf[2] != 'L' || buf[3] != 'F' {
			return fmt.Errorf("downloaded file is not a valid ELF binary")
		}
	case "darwin":
		valid := (buf[0] == 0xca && buf[1] == 0xfe && buf[2] == 0xba && buf[3] == 0xbe) || // fat binary
			(buf[0] == 0xcf && buf[1] == 0xfa && buf[2] == 0xed && buf[3] == 0xfe) || // 64-bit LE
			(buf[0] == 0xce && buf[1] == 0xfa && buf[2] == 0xed && buf[3] == 0xfe) || // 32-bit LE
			(buf[0] == 0xfe && buf[1] == 0xed && buf[2] == 0xfa && buf[3] == 0xcf) || // 64-bit BE
			(buf[0] == 0xfe && buf[1] == 0xed && buf[2] == 0xfa && buf[3] == 0xce) // 32-bit BE
		if !valid {
			return fmt.Errorf("downloaded file is not a valid Mach-O binary")
		}
	case "windows":
		if buf[0] != 'M' || buf[1] != 'Z' {
			return fmt.Errorf("downloaded file is not a valid PE binary")
		}
	}
	return nil
}

// verifyChecksumAndHash fetches checksums.txt + checksums.txt.sig for
// version, verifies the signature with the embedded public key, extracts the
// expected sha256 for binaryName, and compares it against binaryPath's
// actual sha256.
func verifyChecksumAndHash(binaryPath, binaryName, version string) error {
	checksums, err := fetchReleaseFile(version, "checksums.txt")
	if err != nil {
		return fmt.Errorf("failed to fetch checksums.txt: %w", err)
	}

	sig, err := fetchReleaseFile(version, "checksums.txt.sig")
	if err != nil {
		return fmt.Errorf("failed to fetch checksums.txt.sig: %w", err)
	}

	if err := VerifyChecksums(checksums, strings.TrimSpace(string(sig))); err != nil {
		return err
	}

	expected, err := parseChecksum(string(checksums), binaryName)
	if err != nil {
		return err
	}

	actual, err := hashFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to hash downloaded binary: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// fetchReleaseFile downloads a file from the given release.
func fetchReleaseFile(version, filename string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, filename)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("%s not found (404)", filename)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s returned %d", filename, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// parseChecksum extracts the SHA256 hash for binaryName from sha256sum-
// formatted text ("hash  filename" per line).
func parseChecksum(checksums, binaryName string) (string, error) {
	remaining := checksums
	for len(remaining) > 0 {
		var line string
		if i := strings.IndexByte(remaining, '\n'); i >= 0 {
			line = remaining[:i]
			remaining = remaining[i+1:]
		} else {
			line = remaining
			remaining = ""
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		i := strings.IndexByte(line, ' ')
		if i == -1 {
			continue
		}
		hash := line[:i]

		rest := line[i+1:]
		j := 0
		for j < len(rest) && rest[j] == ' ' {
			j++
		}
		if j == len(rest) {
			continue
		}
		name := strings.TrimPrefix(rest[j:], "*")
		if name == binaryName {
			return hash, nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", binaryName)
}

// replaceBinary replaces dst with src. On Windows the running exe can't be
// overwritten directly, so it's renamed aside first and restored on failure.
// On POSIX, os.Rename works even on a running binary.
func replaceBinary(src, dst string) error {
	if runtime.GOOS == "windows" {
		oldPath := dst + ".old"
		os.Remove(oldPath)
		if err := os.Rename(dst, oldPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			os.Rename(oldPath, dst) // restore
			return err
		}
		os.Remove(oldPath)
		return nil
	}

	return os.Rename(src, dst)
}

// hashFile computes the SHA256 hex digest of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
