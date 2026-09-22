package notify

import "fmt"

// ExpiryMessage renders the notification text for a var that is expiring or expired.
func ExpiryMessage(key string, days int) string {
	switch {
	case days < 0:
		return key + " has already expired"
	case days == 0:
		return key + " expires today"
	case days == 1:
		return key + " expires tomorrow"
	default:
		return fmt.Sprintf("%s expires in %d days", key, days)
	}
}
