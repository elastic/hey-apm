package compose

import (
	"bytes"
)

func Compose(numErrors, numTransactions, numSpans, numFrames int) []byte {
	//TODO: shuffle body except for metadata
	var buf bytes.Buffer

	buf.Write(Metadata)
	buf.WriteByte('\n')

	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	span := make([]byte, len(SingleSpan))
	copy(span, SingleSpan)
	span = bytes.Replace(span, []byte(`"stacktrace":[],`), frames, -1)

	transaction := ndjsonWrapObj("transaction", SingleTransaction)
	span = ndjsonWrapObj("span", span)

	for i := 0; i < numTransactions; i++ {
		NDJSONRepeat(&buf, transaction, 1)
		NDJSONRepeat(&buf, span, numSpans)
	}

	errEvent := make([]byte, len(SingleError))
	copy(errEvent, SingleError)
	errEvent = bytes.Replace(errEvent, []byte(`"stacktrace":[],`), frames, -1)

	errEvent = ndjsonWrapObj("error", errEvent)
	NDJSONRepeat(&buf, errEvent, numErrors)

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

func ndjsonWrapObj(key string, buf []byte) []byte {
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
