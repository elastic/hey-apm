package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"

	"github.com/elastic/hey-apm/compose"
	"github.com/elastic/hey-apm/target"
)

var (
	numSpans  = 10
	numFrames = 10
)

func init() {
	if spansEnv, ok := os.LookupEnv("SPANS"); ok {
		if n, err := strconv.Atoi(spansEnv); err == nil {
			numSpans = n
		}
	}
	if framesEnv, ok := os.LookupEnv("FRAMES"); ok {
		if n, err := strconv.Atoi(framesEnv); err == nil {
			numFrames = n
		}
	}

	// make a span with frames
	singleSpan := compose.V2Span(numFrames)
	spans := make([][]byte, numSpans)
	for i := 0; i < numSpans; i++ {
		spans[i] = singleSpan
	}
	// make a list of spans
	spanList := []byte(`"spans":[`)
	spanList = append(spanList, bytes.Join(spans, []byte(","))...)
	spanList = append(spanList, []byte("],")...)

	// make transaction with spans
	singleTransaction := make([]byte, len(compose.SingleTransaction))
	copy(singleTransaction, compose.SingleTransaction)
	singleTransaction = bytes.Replace(singleTransaction, []byte(`"spans":[],`), spanList, -1)
	var transaction interface{}
	if err := json.Unmarshal(singleTransaction, &transaction); err != nil {
		panic(err)
	}

	// make a payload with a transaction
	v2comp := make(map[string]interface{})
	json.Unmarshal(compose.Metadata, &v2comp)
	v2comp = v2comp["metadata"].(map[string]interface{})
	v2comp["transactions"] = []interface{}{transaction}

	body, err := json.Marshal(v2comp)
	if err != nil {
		panic(err)
	}
	Register("v2comp", []target.Target{
		{"POST", "v1/transactions", body, 1, 0},
	})
}
