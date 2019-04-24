package numbers

func Perct(i1, i2 uint64) float64 {
	return Div(i1*100, i1+i2)
}

func Div(i1, i2 uint64) float64 {
	return float64(i1) / float64(i2)
}
