package conv

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/elastic/hey-apm/types"
)

// shamelessly stolen from http://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func ByteCountDecimal(z int64) string {
	n := int64(math.Abs(float64(z)))
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d n", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	var neg string
	if z != n {
		neg = "-"
	}
	return fmt.Sprintf("%s%.1f%cb", neg, float64(n)/float64(div), "kMGTPE"[exp])
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
	case uint64, int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	case []string:
		return strings.Join(v.([]string), ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func ToFloat64(i interface{}) float64 {
	switch x := i.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	default:
		f, err := strconv.ParseFloat(fmt.Sprintf("%d", x), 64)
		if err != nil {
			panic(err)
		}
		return f
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
