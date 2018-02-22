package target

import (
	"io/ioutil"
	"math"
	"net/http"

	"github.com/graphaelli/hey/requester"
	"bytes"
	"time"
)

// Config holds global work configuration
type Config struct {
	MaxRequests, RequestTimeout                             int
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
		MaxRequests:    math.MaxInt32,
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
		body := bytes.Replace(t.Body, []byte("2018-01-09T03:35:37.604813Z"), []byte(time.Now().UTC().Format(time.RFC3339)), -1)
		work[i] = &requester.Work{
			Request:            req,
			RequestBody:        body,
			N:                  cfg.MaxRequests,
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
