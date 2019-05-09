package strcoll

import "strings"

// returns the element of the slice at the given index or the empty string
func Get(idx int, slice []string) string {
	if slice != nil && len(slice) > idx {
		return slice[idx]
	}
	return ""
}

// returns all the elements from the slice starting at the give index, or an empty slice
func From(idx int, slice []string) []string {
	if slice != nil && len(slice) > idx {
		return slice[idx:]
	}
	return make([]string, 0)
}

func Copy(m map[string][]string) map[string][]string {
	copy := make(map[string][]string)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

// in case of repeated keys, last wins
func Concat(ms ...map[string]string) map[string]string {
	ret := make(map[string]string)
	for _, m := range ms {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}

// returns true if s is contained in xs
func Contains(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func SplitKV(s string, sep string) (string, string) {
	ret := strings.SplitN(s, sep, 2)
	return Get(0, ret), Get(1, ret)
}
