package profile

import (
	"github.com/elastic/hey-apm/target"
)

func init() {
	Register("rum", []target.Target{
		{"POST", "v1/client-side/transactions", target.V1TransactionFrontendMinimal1, 2, 0},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
