package compose

import (
	"bytes"
)

// Composes a request body for the v1/transactions endpoint with as many transactions as
// `numTransactions`, each containing as many spans as `numSpans`, each containing as many
// frames as `numFrames` * 10
func TransactionRequest(numTransactions int, numSpans int, numFrames int) []byte {
	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	span := make([]byte, len(SingleSpan))
	copy(span, SingleSpan)
	span = bytes.Replace(span, []byte(`"stacktrace":[],`), frames, -1)
	spans := multiply([]byte(`"spans"`), span, numSpans)

	transaction := make([]byte, len(SingleTransaction))
	copy(transaction, SingleTransaction)
	transaction = bytes.Replace(transaction, []byte(`"spans":[],`), spans, -1)
	transactions := multiply([]byte(`"transactions"`), transaction, numTransactions)

	ret := make([]byte, len(TransactionsPayload))
	copy(ret, TransactionsPayload)
	ret = bytes.Replace(ret, []byte(`"transactions":[],`), transactions, -1)
	return ret
}

// Composes a request body for the v1/errors endpoint with as many errors as
// `numErrors`, each containing as many frames as `numFrames` * 10
func ErrorRequest(numErrors int, numFrames int) []byte {
	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	error := make([]byte, len(SingleError))
	copy(error, SingleError)
	error = bytes.Replace(error, []byte(`"stacktrace":[],`), frames, -1)
	errors := multiply([]byte(`"errors"`), error, numErrors)

	ret := make([]byte, len(ErrorsPayload))
	copy(ret, ErrorsPayload)
	ret = bytes.Replace(ret, []byte(`"errors":[],`), errors, -1)

	return ret
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
