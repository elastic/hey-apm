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
func V2TransactionRequest(numTransactions int, numSpans int, numFrames int) [][]byte {
	payloads := createPayloads()
	transaction := addNewline(NdjsonWrapObj("transaction", SingleTransaction))
	span := addNewline(NdjsonWrapObj("span", V2Span(numFrames)))
	for i := 0; i < numTransactions; i++ {
		payloads = append(payloads, transaction)
		for j := 0; j < numSpans; j++ {
			payloads = append(payloads, span)
		}
	}
	return payloads
}

// Composes a request body for the v2/errors endpoint with as many errors as
// `numErrors`, each containing as many frames as `numFrames` * 10
func V2ErrorRequest(numErrors int, numFrames int) [][]byte {
	payloads := createPayloads()

	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	event := make([]byte, len(SingleError))
	copy(event, SingleError)
	event = bytes.Replace(event, []byte(`"stacktrace":[],`), frames, -1)

	for i := 0; i < numErrors; i++ {
		payloads = append(payloads, event)
	}
	return payloads
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

func createPayloads() [][]byte {
	return [][]byte{addNewline(Metadata)}
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

func addNewline(p []byte) []byte {
	return append(p, '\n')
}
