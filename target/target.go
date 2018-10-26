package target

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/hey-apm/compose"
	"github.com/simitt/hey/requester"
)

const defaultUserAgent = "hey-apm/1.0"

// Config holds global work configuration
type Config struct {
	NumAgents      int
	Qps            float64
	MaxRequests    int
	RequestTimeout int
	RunTimeout     time.Duration
	Endpoint       string

	BodyConfig *BodyConfig

	DisableCompression, DisableKeepAlives, DisableRedirects bool
	http.Header
	Stream bool
}

type BodyConfig struct {
	NumErrors, NumTransactions, NumSpans, NumFrames int
}

type Target struct {
	Url    string
	Method string
	Body   []byte
	Config *Config
}

var (
	defaultCfg = Config{
		MaxRequests:    math.MaxInt32,
		RequestTimeout: 10,
		Endpoint:       "/intake/v2/events",
	}
)

func BuildBody(b *BodyConfig) []byte {
	return compose.Compose(b.NumErrors, b.NumTransactions, b.NumSpans, b.NumFrames)
}

func NewTarget(baseUrl, method string, cfg *Config) *Target {
	if cfg == nil {
		cfg = &defaultCfg
	}
	body := BuildBody(cfg.BodyConfig)
	url := strings.TrimSuffix(baseUrl, "/") + cfg.Endpoint
	return &Target{Config: cfg, Body: body, Url: url, Method: method}
}

// Get constructs the list of work to be completed
func (t *Target) GetWork() *requester.Work {
	// request
	req, err := http.NewRequest(t.Method, t.Url, nil)
	if err != nil {
		panic(err)
	}

	// headers
	for header, values := range t.Config.Header {
		for _, v := range values {
			req.Header.Add(header, v)
		}
	}
	// Use the defaultUserAgent unless the Header contains one, which may be blank to not send the header.
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Add("User-Agent", defaultUserAgent)
	}

	if len(t.Body) > 0 {
		req.Header.Add("Content-Type", "application/x-ndjson")
	}

	report := ioutil.Discard

	if !t.Config.DisableCompression {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		if _, err := gz.Write([]byte(t.Body)); err != nil {
			panic(err)
		}
		if err := gz.Close(); err != nil {
			panic(err)
		}
		t.Body = b.Bytes()
		req.Header.Add("Content-Encoding", "gzip")
	}

	req.ContentLength = int64(len(t.Body))

	workReq := &requester.SimpleReq{
		Request:     req,
		RequestBody: t.Body,
		Timeout:     t.Config.RequestTimeout,
		QPS:         t.Config.Qps,
	}

	return &requester.Work{
		Req:                workReq,
		N:                  t.Config.MaxRequests,
		C:                  t.Config.NumAgents,
		DisableCompression: t.Config.DisableCompression,
		DisableKeepAlives:  t.Config.DisableKeepAlives,
		DisableRedirects:   t.Config.DisableRedirects,
		H2:                 false,
		ProxyAddr:          nil,
		Writer:             report,
	}
}
