package target

import (
	"io/ioutil"
	"math"
	"net/http"

	"github.com/graphaelli/hey/requester"
)

// Config holds global work configuration
type Config struct {
	RequestTimeout                                          int
	DisableCompression, DisableKeepAlives, DisableRedirects bool
}

var (
	targets = []struct {
		method, url string
		body        []byte
		concurrent  int
		qps         float64
	}{
		{"POST", "v1/errors", v1Error1, 3, 0},
		{"POST", "v1/errors", v1Error2, 1, 0},
		{"POST", "v1/transactions", v1Transaction1, 100, 0},
		{"GET", "healthcheck", nil, 2, 0},
	}

	defaultCfg = Config{
		RequestTimeout: 10,
	}
)

// Get constructs the list of work to be completed
func Get(baseUrl string, cfg *Config) []*requester.Work {
	if cfg == nil {
		cfg = &defaultCfg
	}
	work := make([]*requester.Work, len(targets))
	for i, t := range targets {
		url := baseUrl + t.url
		req, err := http.NewRequest(t.method, url, nil)
		if err != nil {
			panic(err)
		}
		if t.body != nil {
			req.Header.Add("Content-Type", "application/json")
		}

		/*
			report, err := os.Create(fmt.Sprintf("%d-%s", i, strings.Replace(filepath.Clean(t.url), "/", "_", -1)))
			if err != nil {
				panic(err)
			}
		*/
		report := ioutil.Discard

		work[i] = &requester.Work{
			Request:            req,
			RequestBody:        t.body,
			N:                  math.MaxInt32,
			C:                  t.concurrent,
			QPS:                t.qps,
			Timeout:            cfg.RequestTimeout,
			DisableCompression: cfg.DisableCompression,
			DisableKeepAlives:  cfg.DisableKeepAlives,
			DisableRedirects:   cfg.DisableRedirects,
			H2:                 false,
			ProxyAddr:          nil,
			Writer:             report,
		}
	}
	return work
}
