package conv

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/elastic/hey-apm/types"
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

func StringOf(v interface{}) string {
	switch v.(type) {
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	case []string:
		return strings.Join(v.([]string), ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func AsFloat64(m interface{}, k string) float64 {
	return asType(m, k, float64(0)).(float64)
}

func AsUint64(m interface{}, k string) uint64 {
	return uint64(AsFloat64(m, k))
}

func AsSlice(m interface{}, k string) types.Is {
	return asType(m, k, make(types.Is, 0)).(types.Is)
}

func AsString(m interface{}, k string) string {
	return asType(m, k, "").(string)
}

func asType(m interface{}, k string, v interface{}) interface{} {
	if m2, ok := m.(types.M); ok {
		if v2, ok := m2[k]; ok {
			if reflect.TypeOf(v2) == reflect.TypeOf(v) {
				return v2
			}
		}
	}
	return v
}
