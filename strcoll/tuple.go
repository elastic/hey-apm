package strcoll

import (
	"strings"

	"github.com/elastic/hey-apm/conv"
)

type Tuple struct {
	First, Second string
}

type Tuples []Tuple

func (ts Tuples) Append(first string, second interface{}) Tuples {
	return append(ts, Tuple{first, conv.StringOf(second)})
}

func (ts Tuples) Format(padding int) string {
	lines := make([]string, 0)
	for _, t := range ts {
		first, second := t.First, t.Second
		first += " " + strings.Repeat(".", padding-len(first)) + " "
		lines = append(lines, first+second)
	}
	return strings.Join(lines, "\n")
}
