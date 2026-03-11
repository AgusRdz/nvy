//go:build !windows

package notify

import (
	"fmt"

	"github.com/gen2brain/beeep"
)

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
	return beeep.Notify("nvy", msg, "")
}
