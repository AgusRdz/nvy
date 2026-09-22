//go:build !windows

package notify

import (
	"github.com/gen2brain/beeep"
)

func Expiring(key string, daysLeft int) error {
	msg := ExpiryMessage(key, daysLeft)
	return beeep.Notify("nvy", msg, "")
}
