package strcoll

import "strings"

// Get returns the element of the slice at the given index or the empty string.
func Get(idx int, slice []string) string {
	if slice != nil && len(slice) > idx {
		return slice[idx]
	}
	return ""
}

// Contains returns true if s is contained in xs.
func Contains(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// SplitKV splits strings that look like username:password.
func SplitKV(s string, sep string) (string, string) {
	ret := strings.SplitN(s, sep, 2)
	return Get(0, ret), Get(1, ret)
}
