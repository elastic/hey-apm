package util

import (
	"strconv"
	"time"
)

// parses command `option` as string
// returns the original cmd without the options, and the option separately
func ParseCmdStringOption(cmd []string, option, dfault string) ([]string, string) {
	return parseCmdOption(cmd, option, dfault, false)
}

// parses command `option` as int
// returns the original cmd without the options, and the option separately
func ParseCmdIntOption(cmd []string, option string, dfault int) ([]string, int) {
	cmd, value := parseCmdOption(cmd, option, strconv.Itoa(dfault), false)
	ret, err := strconv.Atoi(value)
	if err != nil {
		ret = dfault
	}
	return cmd, ret
}

// parses command `option` as bool
// returns the original cmd without the options, and the option separately
func ParseCmdBoolOption(cmd []string, option string) ([]string, bool) {
	cmd, value := parseCmdOption(cmd, option, "", true)
	return cmd, value == option
}

// parses command `option` as duration
// returns the original cmd without the options, and the option separately
func ParseCmdDurationOption(cmd []string, option string, dfault time.Duration) ([]string, time.Duration) {
	cmd, value := parseCmdOption(cmd, option, dfault.String(), false)
	ret, err := time.ParseDuration(value)
	if err != nil {
		ret = dfault
	}
	return cmd, ret
}

// searches `option` in `cmd`, "consuming" `cmd`.
// `isBool` is true if `option` is a boolean (ie doesn't have a explicit value in the next token)
// returns a tuple with a string, and the consumed `cmd` array:
// if `option` is not in `cmd`, the returned string is `dfault` unmodified
// otherwise, if `isBool` is false, the returned string is the one after the `option`
// otherwise (`isBool` is true), the returned string is `option`
//
// chaining `parseCmdStringOption` calls allows any options to be passed in any order
func parseCmdOption(cmd []string, option, dfault string, isBool bool) ([]string, string) {
	for idx, arg := range cmd {
		if arg == option {
			// if the option flag is not boolean, the next string represents its value
			var subIdx int
			if !isBool {
				subIdx = 1
			}
			ret := Get(idx+subIdx, cmd)
			return append(cmd[:idx], From(idx+1+subIdx, cmd)...), ret
		}
	}
	return cmd, dfault
}
