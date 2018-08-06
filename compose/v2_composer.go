package compose

import (
	"bytes"
)

func V2Span(numFrames int) []byte {
	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	span := make([]byte, len(SingleSpan))
	copy(span, SingleSpan)
	return bytes.Replace(span, []byte(`"stacktrace":[],`), frames, -1)
}

// Composes a request body for the v2/intake endpoint with as many transactions as
// `numTransactions` and as many spans as `numSpans` for each, each span containing as many
// frames as `numFrames` * 10
func V2TransactionRequest(numTransactions int, numSpans int, numFrames int) []byte {
	var buf bytes.Buffer

	buf.Write(Metadata)
	buf.WriteByte('\n')

	transaction := NdjsonWrapObj("transaction", SingleTransaction)
	span := NdjsonWrapObj("span", V2Span(numFrames))

	for i := 0; i < numTransactions; i++ {
		NDJSONRepeat(&buf, transaction, 1)
		NDJSONRepeat(&buf, span, numSpans)
	}

	return buf.Bytes()
}

// Composes a request body for the v1/errors endpoint with as many errors as
// `numErrors`, each containing as many frames as `numFrames` * 10
func V2ErrorRequest(numErrors int, numFrames int) []byte {
	var buf bytes.Buffer

	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	error := make([]byte, len(SingleError))
	copy(error, SingleError)
	error = bytes.Replace(error, []byte(`"stacktrace":[],`), frames, -1)

	NDJSONRepeat(&buf, error, numErrors)

	return buf.Bytes()
}

func NDJSONRepeat(buf *bytes.Buffer, value []byte, times int) {
	for i := 0; i < times; i++ {
		_, err := buf.Write(value)
		if err != nil {
			panic(err)
		}
		err = buf.WriteByte('\n')
		if err != nil {
			panic(err)
		}
	}
}

func NdjsonWrapObj(key string, buf []byte) []byte {
	var buff bytes.Buffer

	buff.WriteString(`{"`)
	buff.WriteString(key)
	buff.WriteString(`": `)

	// remove newlines
	buf = bytes.Replace(buf, []byte("\n"), []byte{}, -1)
	buff.Write(buf)
	buff.WriteString(`}`)
	return buff.Bytes()
}
