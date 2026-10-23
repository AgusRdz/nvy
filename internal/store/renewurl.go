package store

import (
	"fmt"
	"net/url"
)

// ValidateRenewURL accepts only absolute http/https URLs with a host, so an
// arbitrary string is never handed to the OS opener.
func ValidateRenewURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid renew URL %q: expected an absolute http(s) URL", raw)
	}
	return nil
}
