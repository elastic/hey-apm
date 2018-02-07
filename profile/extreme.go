package profile

import (
	"encoding/json"

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

	Register("small", []target.Target{
		{"POST", "v1/transactions", fab(2, 1, target.V1Transaction1), 3, 1},
		{"GET", "healthcheck", nil, 1, 1},
	})
	Register("huge", []target.Target{
		{"POST", "v1/transactions", fab(2, 500, target.V1Transaction1), 3, 1},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
