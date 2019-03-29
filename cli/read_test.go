package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type m map[string][]string

func TestVariableInterpolation(t *testing.T) {
	for _, test := range []struct {
		input, output []string
		hasErr        bool
	}{
		{
			[]string{},
			[]string{},
			false,
		},
		{
			[]string{"a"},
			[]string{"a"},
			false,
		},
		{
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
			false,
		},
		{
			[]string{"$", ""},
			[]string{""},
			false,
		},
		{
			[]string{"$", "$", "a", "b"},
			[]string{"a", "b"},
			false,
		},
		{
			[]string{"$", "b", "$", "d", "a", "c"},
			[]string{"a", "b", "c", "d"},
			false,
		},
		{
			[]string{"$"},
			[]string{"$"},
			true,
		},
		{
			[]string{"$", "$"},
			[]string{"$", "$"},
			true,
		},
		{
			[]string{"a", "$"},
			[]string{"a", "$"},
			true,
		},
		{
			[]string{"a", "$", "$", "b"},
			[]string{"a", "$", "$", "b"},
			true,
		},
	} {
		out, err := interpolateVars(test.input, nil)
		assert.Equal(t, test.output, out)
		assert.Equal(t, test.hasErr, err != nil)
	}
}

func TestStringSubstitution(t *testing.T) {

	for _, test := range []struct {
		defs          m
		input, output []string
		hasErr        bool
	}{
		{
			m{},
			[]string{},
			[]string{},
			false,
		},
		{
			m{},
			[]string{"a", "b"},
			[]string{"a", "b"},
			false,
		},
		{
			m{"a": []string{"b"}},
			[]string{"b"},
			[]string{"b"},
			false,
		},
		{
			m{"a": []string{"b"}},
			[]string{"a"},
			[]string{"b"},
			false,
		},
		{
			m{"a": []string{"b", "c"}},
			[]string{"a", "d"},
			[]string{"b", "c", "d"},
			false,
		},
		{
			m{"a": []string{"b", "c"}, "d": []string{"e", "f"}},
			[]string{"a", "d"},
			[]string{"b", "c", "e", "f"},
			false,
		},
		{
			m{"a": []string{"b", "c"}, "c": []string{"d", "e"}},
			[]string{"c"},
			[]string{"d", "e"},
			false,
		},
		{
			m{"a": []string{"b", "c"}, "c": []string{"d", "e"}},
			[]string{"a"},
			[]string{"b", "d", "e"},
			false,
		},
		{
			m{"a": []string{"b", "c"}, "c": []string{"d", "e"}},
			[]string{"a", "c", "c"},
			[]string{"b", "d", "e", "d", "e", "d", "e"},
			false,
		},
		{
			m{"a": []string{"b"}, "b": []string{"c"}, "c": []string{"d"}},
			[]string{"a"},
			[]string{"d"},
			false,
		},
		{
			m{"a": []string{"b", "c"}, "b": []string{"f", "g"}},
			[]string{"z", "a", "x", "b", "0"},
			[]string{"z", "f", "g", "c", "x", "f", "g", "0"},
			false,
		},

		{
			m{"a": []string{"b"}, "b": []string{"a"}},
			[]string{"a"},
			[]string{"a"},
			true,
		},
	} {
		out, err := substituteN(test.defs, test.input)
		assert.Equal(t, test.output, out)
		assert.Equal(t, test.hasErr, err != nil)
	}
}

func TestRead(t *testing.T) {
	read := read(m{
		"x": []string{"$", "y"},
		"y": []string{"z"},
		"g": []string{"h"},
		"h": []string{"$"},
	},
	)
	for _, test := range []struct {
		input  string
		output [][]string
		errMsg string
	}{
		{
			"",
			[][]string{},
			"",
		},
		{
			"a",
			[][]string{{"a"}},
			"",
		},
		{
			";",
			[][]string{},
			"",
		},
		{
			"a; b",
			[][]string{{"a;", "b"}},
			"",
		},
		{
			"a b ; d",
			[][]string{{"a", "b"}, {"d"}},
			"",
		},
		{
			"a ; y",
			[][]string{{"a"}, {"z"}},
			"",
		},
		{
			"$ a b",
			[][]string{{"b", "a"}},
			"",
		},
		{
			"$ a ; a b",
			[][]string{{"b", "a"}, {"a"}},
			"",
		},
		{
			"a x 1",
			[][]string{{"a", "1", "z"}},
			"",
		},
		{
			"a ; x ; 1",
			[][]string{{"a"}, {"1", "z"}},
			"",
		},
		{
			"a ; x ; 2 1",
			[][]string{{"a"}, {"1", "z"}, {"2"}},
			"",
		},
		{
			"h g 1",
			[][]string{{"$", "$", "1"}},
			"too many variables\n",
		},
		{
			"define x 1 ; x",
			[][]string{{"define", "x", "1", ";", "x"}},
			"",
		},
		{
			"x ; define x 1",
			[][]string{},
			"define can only be the first word in a line\n",
		},
	} {
		out, err := read(test.input, nil)
		assert.Equal(t, test.output, out)
		assert.Equal(t, test.errMsg, errMsg(err))
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
