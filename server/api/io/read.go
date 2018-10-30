package io

import (
	"errors"
	s "strings"

	"github.com/elastic/hey-apm/server/strcoll"
)

// returns a function that takes an string and an error, and returns an slice of slices of strings and an error
// the input string is entered by the user, each output slice is a command
func Read(nameDefs map[string][]string) func(string, error) ([][]string, error) {
	return func(input string, err error) ([][]string, error) {
		ret := make([][]string, 0)
		if err != nil {
			return ret, err
		}
		if s.HasPrefix(input, "define") {
			// do not resolve user-defines names
			return append(ret, s.Fields(input)), nil
		}
		if s.Contains(input, "define") {
			// do not allow recursive definitions
			return ret, errors.New("define can only be the first word in a line\n")
		}
		expr, err := interpolateVars(substituteN(nameDefs, s.Fields(input)))
		// wrap expression with empty strings so that splitting on " ; " works at the string boundaries
		expr = append([]string{""}, append(expr, "")...)
		expr = s.Split(s.Join(expr, " "), " ; ")
		for _, cmd := range expr {
			if fields := s.Fields(cmd); len(fields) > 0 {
				ret = append(ret, fields)
			}
		}
		return ret, err
	}
}

// interpolates strings in positions indicated by $ placeholders
// eg [a b $ d $ f C E] becomes [a b C d E f]
func interpolateVars(expr []string, err error) ([]string, error) {
	if err != nil {
		return expr, err
	}
	ret := make([]string, len(expr))
	for idx, token := range expr {
		ret[idx] = token
	}
	vars := make([]int, 0)
	for idx, token := range ret {
		if token == "$" {
			vars = append(vars, idx)
		}
	}
	if len(vars) > 0 && len(vars) > len(expr[vars[len(vars)-1]+1:]) {
		return ret, errors.New("too many variables\n")
	}

	for idx, v := range vars {
		ret[v] = ret[len(ret)-len(vars)+idx]
	}
	return ret[:len(ret)-len(vars)], nil
}

// substitutes on `expression` strings found in `nameDefs` keys with their respective values
// doesn't do cycle detection, but aborts after 99 substitutions
// doesn't sanitize the input
func substituteN(nameDefs map[string][]string, expression []string) ([]string, error) {
	var err error
	var loop int
	for b := true; b == true && loop < 100; {
		expression, b = substitute1(nameDefs, expression)
		loop++
	}
	if loop == 100 {
		err = errors.New("too many substitutions (recursive definition?)\n")
	}
	return expression, err
}

func substitute1(nameDefs map[string][]string, expression []string) ([]string, bool) {
	for idx, word := range expression {
		if v, ok := nameDefs[word]; ok {
			return append(expression[:idx], append(v, expression[idx+1:]...)...), true
		}
	}
	return expression, false
}

// parses short and long command line options (--, -)
// returns the original cmd without the options, and the options separately
func ParseCmdOptions(cmd []string) ([]string, []string) {
	// order is relevant
	cmd, opts := parseCmdOptions(cmd, "--")
	cmd, shortOpts := parseCmdOptions(cmd, "-")
	for _, opt := range shortOpts {
		opts = append(opts, s.Split(opt, "")...)
	}
	return cmd, opts
}

func parseCmdOptions(cmd []string, prefix string) ([]string, []string) {
	opts := make([]string, 0)
	for {
		cmdNext, opt := ParseCmdOption(cmd, prefix, "", false)
		// when cmd and cmdNext are the same, stop iterating (nothing else will be found)
		if s.Join(cmdNext, " ") == s.Join(cmd, " ") {
			return cmdNext, opts
		}
		cmd = cmdNext
		opts = append(opts, opt)
	}
}

// searches `prefix` in `cmd`, "consuming" `cmd`.
// returns a tuple with the matched string, and the consumed `cmd` array:
// if no string in `cmd` starts with `prefix`, it returns `cmd` and `dfault` untouched, eg:
// 		cmd: {"-m"}, prefix: "-n", dfault:"X" => {"-m"}, "X"
// otherwise, if `next` is true:
// 		the matched string is the string after the one starting with `prefix`, eg:
// 		cmd: {"-n", "42", "-m"}, prefix: "-n" => {"-m"}, "42"
// otherwise (`next` is false):
// 		the matched string is the string starting with `prefix` *without* the prefix, eg:
// 		cmd: {"-n42"}, prefix: "-n" => {}, "42"
//
// chaining `ParseCmdOption` calls allows any options to be passed in any order
func ParseCmdOption(cmd []string, prefix, dfault string, next bool) ([]string, string) {
	for idx, arg := range cmd {
		if s.HasPrefix(arg, prefix) {
			// subIdx and next are for options whose value is provided in the next token
			var subIdx int
			if next {
				subIdx = 1
			}
			ret := strcoll.Nth(idx+subIdx, cmd)
			if !next {
				// if an option is found, return it without the prefix (usually - or --)
				ret = ret[len(prefix):]
			}
			return append(cmd[:idx], strcoll.Rest(idx+1+subIdx, cmd)...), ret
		}
	}
	return cmd, dfault
}
