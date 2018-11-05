package compose

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"time"
)

func Compose(numErrors, numTransactions, numSpans, numFrames int) []byte {
	rand.Seed(time.Now().UnixNano())
	var buf bytes.Buffer

	buf.Write(Metadata)
	buf.WriteByte('\n')

	lenSpans := len(Spans)

	for i := 0; i < numTransactions; i++ {
		ev := randomized(SingleTransaction, 8)
		write(&buf, ndjsonWrapObj("transaction", ev))

		for i := 0; i < numSpans; i++ {
			ev := randomized(Spans[rand.Intn(lenSpans)], 8)
			span := make([]byte, len(ev))
			copy(span, ev)
			span = bytes.Replace(span, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
			write(&buf, ndjsonWrapObj("span", span))
		}
	}

	for i := 0; i < numErrors; i++ {
		ev := randomized(SingleError, 16)
		errEvent := make([]byte, len(ev))
		copy(errEvent, ev)
		errEvent = bytes.Replace(errEvent, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
		write(&buf, ndjsonWrapObj("error", errEvent))
	}

	return bytes.TrimSpace(buf.Bytes())
}

func randomized(event []byte, idN int) []byte {
	var ev map[string]interface{}
	err := json.Unmarshal(event, &ev)
	if err != nil {
		panic(err)
	}
	ev["id"] = randHexString(idN)
	ev["transaction_id"] = randHexString(8)
	ev["parent_id"] = randHexString(8)
	ev["trace_id"] = randHexString(16)
	b, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	return b
}

func randHexString(n int) string {
	buf := make([]byte, n)
	rand.Read(buf)
	return hex.EncodeToString(buf)
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
	l := len(StacktraceFrames)
	var buf bytes.Buffer
	buf.Write([]byte(`"stacktrace": [`))
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')

		}
		randFrame := StacktraceFrames[rand.Intn(l)]
		fr := make([]byte, len(randFrame))
		copy(fr, randFrame)
		buf.Write(fr)
	}
	buf.WriteString("],")
	return buf.Bytes()
}
