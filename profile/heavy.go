package profile

import (
	"github.com/elastic/hey-apm/target"
)

func init() {
	Register("heavy", []target.Target{
		{"POST", "v1/errors", target.V1Error1, 3, 10},
		{"POST", "v1/errors", target.V1Error2, 1, 10},
		{"POST", "v1/transactions", target.V1Transaction1, 50, 10},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
