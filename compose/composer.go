package compose

import (
	"bytes"
)

func Compose(numErrors, numTransactions, numSpans, numFrames int) [][]byte {
	var ret [][]byte

	meta := append([]byte(nil), Metadata...)
	ret = append(ret, append(meta, '\n'))

	stacktrace := make([]byte, len(StacktraceFrame))
	copy(stacktrace, StacktraceFrame)
	frames := multiply([]byte(`"stacktrace"`), stacktrace, numFrames)

	span := make([]byte, len(SingleSpan))
	copy(span, SingleSpan)
	span = bytes.Replace(span, []byte(`"stacktrace": [],`), frames, -1)

	transaction := ndjsonWrapObj("transaction", SingleTransaction)
	span = ndjsonWrapObj("span", span)

	for i := 0; i < numTransactions; i++ {
		ret = append(ret, append([]byte(nil), transaction...))
		for j := 0; j < numSpans; j++ {
			ret = append(ret, append([]byte(nil), span...))
		}
	}

	errEvent := make([]byte, len(SingleError))
	copy(errEvent, SingleError)
	errEvent = bytes.Replace(errEvent, []byte(`"stacktrace": [],`), frames, -1)

	errEvent = ndjsonWrapObj("error", errEvent)
	for i := 0; i < numErrors; i++ {
		ret = append(ret, append([]byte(nil), errEvent...))
	}
	return ret
}

func Concat(bs [][]byte) []byte {
	var buf bytes.Buffer
	for _, ba:= range bs {
		buf.Write(ba)
		buf.WriteByte('\n')
	}
	return bytes.TrimSpace(buf.Bytes())
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
