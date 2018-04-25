package profile

import (
	"github.com/elastic/hey-apm/target"
)

func init() {
	Register("rum", []target.Target{
		{"POST", "v1/client-side/errors", target.V1ErrorFrontendError1, 5, 0},
		{"POST", "v1/client-side/transactions", target.V1ErrorFrontendTransaction1, 10, 0},
		{"GET", "healthcheck", nil, 1, 1},
	})

	Register("rumtiny", []target.Target{
		{"POST", "v1/client-side/errors", target.V1ErrorFrontendMinimal1, 2, 0},
		{"POST", "v1/client-side/transactions", target.V1TransactionFrontendTiming1, 2, 0},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
