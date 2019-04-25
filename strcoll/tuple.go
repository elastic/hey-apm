package strcoll

import (
	"strings"

	"github.com/elastic/hey-apm/conv"
)

type Tuple struct {
	First, Second string
}

type Tuples struct {
	data []Tuple
}

func NewTuples() Tuples {
	return Tuples{data: make([]Tuple, 0)}
}

func (ts *Tuples) Add(first string, second interface{}) {
	ts.data = append(ts.data, Tuple{first, conv.StringOf(second)})
}

func (ts Tuples) Format(padding int) string {
	lines := make([]string, 0)
	for _, t := range ts.data {
		first, second := t.First, t.Second
		first += " " + strings.Repeat(".", padding-len(first)) + " "
		lines = append(lines, first+second)
	}
	return strings.Join(lines, "\n")
}
