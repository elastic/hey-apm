package numbers

import (
	"math"

	"github.com/elastic/hey-apm/conv"
)

// CPerct returns the complement percentage of one value of another, or nil if it can't be calculated.
func CPerct(i1, i2 uint64) *float64 {
	f := Div(i1*100, i1+i2)
	if f != nil {
		c := 100 - *f
		return &c
	}
	return nil
}

// CPerct returns the percentage of one value of another.
func Perct(i1, i2 uint64) *float64 {
	return Div(i1*100, i1+i2)
}

// Div divides any 2 integer numeric types and returns a float64 with 2 precision.
func Div(i1, i2 interface{}) *float64 {
	return Divp(i1, i2, 2)
}

// Divp divides any 2 integer numeric types and returns a float64 with given precision.
func Divp(i1, i2 interface{}, p int) *float64 {
	f1 := conv.ToFloat64(i1)
	f2 := conv.ToFloat64(i2)
	if f2 == 0 {
		return nil
	}
	f := truncate(f1/f2, p)
	return &f
}

func truncate(f float64, p int) float64 {
	m := math.Pow10(p)
	ret := math.Round(f*m) / m
	// workaround lack of elasticsearch mapping
	// don't tell this anyone
	if ret == float64(int64(ret)) {
		ret += 0.01
	}
	return ret
}

// Sum sums all its arguments.
func Sum(xs ...uint64) uint64 {
	var i uint64
	for _, x := range xs {
		i += x
	}
	return i
}

// SumPt sums 2 pointers and returns a pointer to the result
func SumPt(i1 *int64, i2 *int64) *int64 {
	if i1 == nil && i2 == nil {
		return nil
	}
	if i1 == nil {
		return i2
	}
	if i2 == nil {
		return i1
	}
	i := *i1 + *i2
	return &i
}

// IntDivPt makes an integer division where the numerator is a pointer,
// and returns a pointer to the result
func IntDivPt(n *int64, d int) *int64 {
	if n == nil || d == 0 {
		return nil
	}
	i := *n / int64(d)
	return &i
}
