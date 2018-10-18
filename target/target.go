package target

import (
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"strings"

	"github.com/graphaelli/hey/requester"
)

const defaultUserAgent = "hey-apm/1.0"

// Config holds global work configuration
type Config struct {
	MaxRequests, RequestTimeout                             int
	DisableCompression, DisableKeepAlives, DisableRedirects bool
	http.Header
}

type Target struct {
	Method, Url string
	Body        [][]byte
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
		pReader, pWriter := io.Pipe()
		req, err := http.NewRequest(t.Method, url, pReader)
		if err != nil {
			panic(err)
		}

		// add global headers
		for header, values := range cfg.Header {
			for _, v := range values {
				req.Header.Add(header, v)
			}
		}
		// Use the defaultUserAgent unless the Header contains one, which may be blank to not send the header.
		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Add("User-Agent", defaultUserAgent)
		}

		if t.Body != nil {
			if strings.HasPrefix(t.Url, "/intake/v2/") {
				req.Header.Add("Content-Type", "application/x-ndjson")
			} else {
				req.Header.Add("Content-Type", "application/json")
			}
		}

		report := ioutil.Discard
		if !cfg.DisableCompression {
			req.Header.Add("Content-Encoding", "gzip")
		}
		work[i] = &requester.Work{
			Request:            req,
			RequestBody:        t.Body,
			PipeWriter:         pWriter,
			PipeReader:         pReader,
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
