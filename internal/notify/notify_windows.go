//go:build windows

package notify

import (
	"github.com/go-toast/toast"
)

// Expiring fires a Windows toast notification using a registered AppID
// so the app name shows correctly instead of "DefaultAppName". With a renew
// URL the toast body and a "Renew" button both open it.
func Expiring(key string, daysLeft int, renewURL string) error {
	msg := ExpiryMessage(key, daysLeft)

	n := toast.Notification{
		// Use the Windows PowerShell AppID — registered on all Windows systems.
		AppID:   "{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\\WindowsPowerShell\\v1.0\\powershell.exe",
		Title:   "nvy",
		Message: msg,
	}
	if renewURL != "" {
		n.ActivationType = "protocol"
		n.ActivationArguments = renewURL
		n.Actions = []toast.Action{{Type: "protocol", Label: "Renew", Arguments: renewURL}}
	}
	return n.Push()
}
