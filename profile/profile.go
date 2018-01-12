package profile

import (
	"sort"

	"github.com/elastic/hey-apm/target"
)

var (
	profiles = make(map[string]target.Targets)
)

// Choices lists the profile names
func Choices() []string {
	choices := make([]string, len(profiles))
	i := 0
	for k := range profiles {
		choices[i] = k
		i++
	}
	sort.Strings(choices)
	return choices
}

// Get fetches a registered profile by name
func Get(name string) target.Targets {
	return profiles[name]
}

// Register a profile by name
func Register(name string, t target.Targets) {
	profiles[name] = t
}
