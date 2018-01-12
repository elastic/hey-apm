package profile

import (
	"github.com/elastic/hey-apm/target"
)

// Model a bad deployment - high % and count of errors + some transactions.

func init() {
	Register("baddeploy", []target.Target{
		{"POST", "v1/errors", target.V1Error1, 50, 10},
		{"POST", "v1/errors", target.V1Error2, 50, 10},
		{"POST", "v1/transactions", target.V1Transaction1, 2, 10},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
