package profile

import (
	"encoding/json"

	"github.com/elastic/hey-apm/target"
)

func init() {
	duplicateSpans := func(count int, b []byte) []byte {
		var template map[string]interface{}
		if err := json.Unmarshal(b, &template); err != nil {
			panic(err)
		}

		span0 := template["transactions"].([]interface{})[0].(map[string]interface{})["spans"].([]interface{})[0].(map[string]interface{})
		dupSpans := make([]interface{}, count)
		for i := 0; i < count; i++ {
			span := map[string]interface{}{}
			for k, v := range span0 {
				span[k] = v
			}
			span["id"] = interface{}(i)
			dupSpans[i] = span
		}

		template["transactions"].([]interface{})[0].(map[string]interface{})["spans"] = dupSpans
		c, err := json.Marshal(template)
		if err != nil {
			panic(err)
		}
		return c
	}

	Register("maxspans", []target.Target{
		{"POST", "v1/transactions", duplicateSpans(500, target.V1Transaction1), 4, 10},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
