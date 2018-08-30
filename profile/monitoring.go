package profile

import (
	"github.com/elastic/hey-apm/target"
)

func init() {
	Register("monitoring/bad_json", []target.Target{
		{"POST", "v1/transactions", []byte("invalid json"), 2, 10},
	})
	Register("monitoring/invalid_transaction", []target.Target{
		{"POST", "v1/transactions", []byte("{}"), 2, 10},
	})
	Register("monitoring/wrong_method", []target.Target{
		{"GET", "v1/transactions", target.V1Transaction2, 2, 10},
	})
}
