package util

import (
	"errors"
	"fmt"
	"strconv"
)

// shamelessly stolen from http://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d b", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cb", float64(b)/float64(div), "kMGTPE"[exp])
}

// like Atoi for positive integers and error handling
func Aton(attr string, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(attr)
	if n < 0 {
		err = errors.New("negative values not allowed")
	}
	return n, err
}
