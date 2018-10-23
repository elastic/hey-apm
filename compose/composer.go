package compose

import (
	"bytes"
)

// Deprecated, see v2_composer
func TransactionRequest(numTransactions int, numSpans int, numFrames int) []byte {
	return nil
}

// Deprecated, see v2_composer
func ErrorRequest(numErrors int, numFrames int) []byte {
	return nil
}

func multiply(key []byte, value []byte, times int) []byte {
	var ret bytes.Buffer
	var c int
	ret.Write(key)
	ret.WriteString(":[")
	ret.Write(value)
	for c < times-1 {
		ret.WriteByte(',')
		ret.Write(value)
		c++
	}
	ret.WriteString("],")
	return ret.Bytes()
}
