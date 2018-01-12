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

type Target struct {
	Method, Url string
	Body        []byte
	Concurrent  int
	Qps         float64
}

type Targets []Target

var (
	defaultCfg = Config{
		RequestTimeout: 10,
	}
)

// Get constructs the list of work to be completed
func (targets Targets) GetWork(baseUrl string, cfg *Config) []*requester.Work {
	if cfg == nil {
		cfg = &defaultCfg
	}
	work := make([]*requester.Work, len(targets))
	for i, t := range targets {
		url := baseUrl + t.Url
		req, err := http.NewRequest(t.Method, url, nil)
		if err != nil {
			panic(err)
		}
		if t.Body != nil {
			req.Header.Add("Content-Type", "application/json")
		}

		report := ioutil.Discard

		work[i] = &requester.Work{
			Request:            req,
			RequestBody:        t.Body,
			N:                  math.MaxInt32,
			C:                  t.Concurrent,
			QPS:                t.Qps,
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
