package numbers

import (
	"math"

	"github.com/elastic/hey-apm/conv"
)

// complement percentage
func CPerct(i1, i2 uint64) *float64 {
	f := Div(i1*100, i1+i2)
	if f != nil {
		c := 100 - *f
		return &c
	}
	return nil
}

func Perct(i1, i2 uint64) *float64 {
	return Div(i1*100, i1+i2)
}

func Div(i1, i2 interface{}) *float64 {
	return Divp(i1, i2, 2)
}

// divides any 2 integer numeric types and returns a float64 with given precision
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

func Sum(xs ...uint64) uint64 {
	var i uint64
	for _, x := range xs {
		i += x
	}
	return i
}
