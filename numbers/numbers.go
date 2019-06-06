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
	return math.Round(f*m) / m
}

// Sum sums all its arguments.
func Sum(xs ...uint64) uint64 {
	var i uint64
	for _, x := range xs {
		i += x
	}
	return i
}
