//go:build !windows

package notify

import (
	"github.com/gen2brain/beeep"
)

// Expiring fires a desktop notification. beeep can't carry click actions, so a
// renew URL is surfaced as an `nvy renew` hint instead.
func Expiring(key string, daysLeft int, renewURL string) error {
	msg := ExpiryMessage(key, daysLeft) + RenewHint(key, renewURL)
	return beeep.Notify("nvy", msg, "")
}
