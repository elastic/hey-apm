package profile

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/hey-apm/target"
)

func init() {
	fab := func(transactionCount int, spanCount int, b []byte) []byte {
		var template map[string]interface{}
		if err := json.Unmarshal(b, &template); err != nil {
			panic(err)
		}

		span0 := template["transactions"].([]interface{})[0].(map[string]interface{})["spans"].([]interface{})[0].(map[string]interface{})
		dupSpans := make([]interface{}, spanCount)
		for i := 0; i < spanCount; i++ {
			span := map[string]interface{}{}
			for k, v := range span0 {
				span[k] = v
			}
			span["id"] = interface{}(i)
			dupSpans[i] = span
		}

		template["transactions"].([]interface{})[0].(map[string]interface{})["spans"] = dupSpans

		transactions := make([]interface{}, transactionCount)
		trans0 := template["transactions"].([]interface{})[0]
		for i := 0; i < transactionCount; i++ {
			transactions[i] = trans0
		}
		template["transactions"] = transactions

		c, err := json.Marshal(template)
		if err != nil {
			panic(err)
		}

		return c
	}

	fiveMB := struct{ transactions, spans int }{1, 286}
	for i := 1; i < 10; i++ {
		Register(fmt.Sprintf("%dmb", i*5), []target.Target{
			{"POST", "v1/transactions", fab(fiveMB.transactions*i, fiveMB.spans, target.V1Transaction1), 100, 0},
		})
	}
	hundredKB := struct{ transactions, spans int }{7, 1} // ~ 130 KB
	for i := 1; i < 8; i++ {
		Register(fmt.Sprintf("%dkb", i*130), []target.Target{
			{"POST", "v1/transactions", fab(hundredKB.transactions*i, hundredKB.spans, target.V1Transaction1), 100, 0},
		})
	}
	Register("1mb", []target.Target{
		{"POST", "v1/transactions", fab(28, 2, target.V1Transaction1), 100, 0},
	})
	Register("huge", []target.Target{
		{"POST", "v1/transactions", fab(2, 500, target.V1Transaction1), 3, 1},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
