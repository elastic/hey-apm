package strcoll

import "strings"

// Get returns the element of the slice at the given index or the empty string.
func Get(idx int, slice []string) string {
	if slice != nil && len(slice) > idx {
		return slice[idx]
	}
	return ""
}

// SplitKV splits strings that look like username:password.
func SplitKV(s string, sep string) (string, string) {
	ret := strings.SplitN(s, sep, 2)
	return Get(0, ret), Get(1, ret)
}
