//go:build windows

package notify

import (
	"fmt"

	"github.com/go-toast/toast"
)

// Expiring fires a Windows toast notification using a registered AppID
// so the app name shows correctly instead of "DefaultAppName".
func Expiring(key string, daysLeft int) error {
	var msg string
	switch {
	case daysLeft < 0:
		msg = key + " has expired"
	case daysLeft == 0:
		msg = key + " expires today"
	default:
		msg = fmt.Sprintf("%s expires in %d days", key, daysLeft)
	}

	n := toast.Notification{
		// Use the Windows PowerShell AppID — registered on all Windows systems.
		AppID:   "{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\\WindowsPowerShell\\v1.0\\powershell.exe",
		Title:   "nvy",
		Message: msg,
	}
	return n.Push()
}
