package strcoll

func Nth(nth int, slice []string) string {
	if slice != nil && len(slice) > nth {
		return slice[nth]
	}
	return ""
}

func Rest(nth int, slice []string) []string {
	if slice != nil && len(slice) > nth {
		return slice[nth:]
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

// returns true if any string in xs2 is contained in xs1
func ContainsAny(xs1 []string, xs2 ...string) bool {
	for _, x := range xs2 {
		if Contains(x, xs1) {
			return true
		}
	}
	return false
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
