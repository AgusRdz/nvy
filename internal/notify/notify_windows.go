//go:build windows

package notify

import (
	"github.com/go-toast/toast"
)

// Expiring fires a Windows toast notification using a registered AppID
// so the app name shows correctly instead of "DefaultAppName". A renew URL is
// surfaced as an `nvy renew` hint rather than a click action, so a stray click
// on the toast never launches a browser.
func Expiring(key string, daysLeft int, renewURL string) error {
	msg := ExpiryMessage(key, daysLeft) + RenewHint(key, renewURL)

	n := toast.Notification{
		// Use the Windows PowerShell AppID — registered on all Windows systems.
		AppID:   "{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\\WindowsPowerShell\\v1.0\\powershell.exe",
		Title:   "nvy",
		Message: msg,
	}
	return n.Push()
}
