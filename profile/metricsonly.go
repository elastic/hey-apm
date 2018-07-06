package profile

import (
	"github.com/elastic/hey-apm/target"
)

func init() {
	Register("metricsonly", []target.Target{
		{"POST", "v1/metrics", target.V1Metric1, 5, 0},
		{"GET", "healthcheck", nil, 1, 1},
	})
}
