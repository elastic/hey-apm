package compose

import (
	"bytes"
	"math/rand"
	"time"
)

func Compose(numErrors, numTransactions, numSpans, numFrames int) []byte {
	rand.Seed(time.Now().UnixNano())
	var buf bytes.Buffer

	buf.Write(metadata)
	buf.WriteByte('\n')

	lenSpans := len(spans)

	for i := 0; i < numTransactions; i++ {
		write(&buf, ndjsonWrapObj("transaction", singleTransaction))

		for i := 0; i < numSpans; i++ {
			ev := spans[rand.Intn(lenSpans)]
			span := make([]byte, len(ev))
			copy(span, ev)
			span = bytes.Replace(span, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
			write(&buf, ndjsonWrapObj("span", span))
		}
	}

	for i := 0; i < numErrors; i++ {
		errEvent := make([]byte, len(singleError))
		copy(errEvent, singleError)
		errEvent = bytes.Replace(errEvent, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
		write(&buf, ndjsonWrapObj("error", errEvent))
	}

	return bytes.TrimSpace(buf.Bytes())
}

func write(buf *bytes.Buffer, value []byte) {
	_, err := buf.Write(value)
	if err != nil {
		panic(err)
	}
	buf.WriteByte('\n')
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

func stacktrace(n int) []byte {
	l := len(stacktraceFrames)
	var buf bytes.Buffer
	buf.Write([]byte(`"stacktrace": [`))
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')

		}
		randFrame := stacktraceFrames[rand.Intn(l)]
		fr := make([]byte, len(randFrame))
		copy(fr, randFrame)
		buf.Write(fr)
	}
	buf.WriteString("],")
	return buf.Bytes()
}
