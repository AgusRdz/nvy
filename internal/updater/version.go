package updater

import "strings"

// IsDev reports whether version looks like a dev build. Self-update is
// skipped for dev builds.
func IsDev(version string) bool {
	return version == "dev" || strings.Contains(version, "-dirty")
}

// IsNewer reports whether a is strictly newer than b. Both are expected in
// "vX.Y.Z" form. A dev build (see IsDev) is never treated as newer than
// anything, and malformed input compares false.
func IsNewer(a, b string) bool {
	if IsDev(a) || IsDev(b) {
		return false
	}
	pa, ok1 := parseSemver(a)
	pb, ok2 := parseSemver(b)
	if !ok1 || !ok2 {
		return false
	}
	if pa[0] != pb[0] {
		return pa[0] > pb[0]
	}
	if pa[1] != pb[1] {
		return pa[1] > pb[1]
	}
	return pa[2] > pb[2]
}

// parseSemver parses a "vX.Y.Z" tag into [major, minor, patch].
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i, p := range parts {
		if p == "" {
			return [3]int{}, false
		}
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return [3]int{}, false
			}
			n = n*10 + int(c-'0')
		}
		nums[i] = n
	}
	return nums, true
}
