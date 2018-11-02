package compose

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"
)

const lenSpans = 7

func Compose(numErrors, numTransactions, numSpans, numFrames int) []byte {
	rand.Seed(time.Now().UnixNano())
	var buf bytes.Buffer

	buf.Write(Metadata)
	buf.WriteByte('\n')

	for i := 0; i < numTransactions; i++ {
		ev := randomized("transaction", 8)
		write(&buf, ndjsonWrapObj("transaction", ev))

		for i := 0; i < numSpans; i++ {
			ev := randomized(fmt.Sprintf("span%d", rand.Intn(lenSpans)+1), 8)
			span := make([]byte, len(ev))
			copy(span, ev)
			span = bytes.Replace(span, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
			write(&buf, ndjsonWrapObj("span", span))
		}
	}

	for i := 0; i < numErrors; i++ {
		ev := randomized("error", 16)
		errEvent := make([]byte, len(ev))
		copy(errEvent, ev)
		errEvent = bytes.Replace(errEvent, []byte(`"stacktrace":[],`), stacktrace(numFrames), -1)
		write(&buf, ndjsonWrapObj("error", errEvent))
	}

	return bytes.TrimSpace(buf.Bytes())
}

func randomized(key string, idN int) []byte {
	f, err := ioutil.ReadFile(fmt.Sprintf("./compose/%s.json", key))
	if err != nil {
		panic(err)
	}
	var ev map[string]interface{}
	json.Unmarshal([]byte(f), &ev)
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
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func write(buf *bytes.Buffer, value []byte) {
	_, err := buf.Write(value)
	if err != nil {
		panic(err)
	}
	err = buf.WriteByte('\n')
	if err != nil {
		panic(err)
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
	ioutil.WriteFile("/tmp/sf.json", buf.Bytes(), 0644)
	return buf.Bytes()
}
