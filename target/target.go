package target

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/hey-apm/compose"
	"github.com/elastic/hey-apm/requester"
)

const defaultUserAgent = "hey-apm/1.0"

type Config struct {
	NumAgents      int
	Throttle       float64
	Pause          time.Duration
	MaxRequests    int
	RequestTimeout time.Duration
	RunTimeout     time.Duration
	Endpoint       string
	Stream         bool
	*BodyConfig
	DisableCompression, DisableKeepAlives, DisableRedirects bool
	http.Header
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
		RequestTimeout: 10 * time.Second,
		Endpoint:       "/intake/v2/events",
		BodyConfig:     &BodyConfig{},
		Header:         make(http.Header),
	}
)

func buildBody(b *BodyConfig) []byte {
	return compose.Compose(b.NumErrors, b.NumTransactions, b.NumSpans, b.NumFrames)
}

func NewTargetFromConfig(baseUrl, method string, cfg *Config) *Target {
	if cfg == nil {
		copyCfg := defaultCfg
		cfg = &copyCfg
	}
	body := buildBody(cfg.BodyConfig)
	url := strings.TrimSuffix(baseUrl, "/") + cfg.Endpoint
	return &Target{Config: cfg, Body: body, Url: url, Method: method}
}

func NewTargetFromOptions(baseUrl string, opts ...OptionFunc) (*Target, error) {
	copyCfg := defaultCfg
	cfg := &copyCfg
	var err error
	for _, opt := range opts {
		err = with(cfg, opt, err)
	}
	body := buildBody(cfg.BodyConfig)
	url := strings.TrimSuffix(baseUrl, "/") + cfg.Endpoint
	return &Target{Config: cfg, Body: body, Url: url, Method: "POST"}, err
}

type OptionFunc func(*Config) error

func with(c *Config, f OptionFunc, err error) error {
	if err != nil {
		return err
	}
	return f(c)
}

func RunTimeout(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.RunTimeout, err = time.ParseDuration(s)
		return err
	}
}

func RequestTimeout(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.RequestTimeout, err = time.ParseDuration(s)
		return err
	}
}

func NumAgents(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.NumAgents, err = strconv.Atoi(s)
		return err
	}
}

func Throttle(s string) OptionFunc {
	return func(c *Config) error {
		throttle, err := strconv.Atoi(s)
		c.Throttle = float64(throttle)
		return err
	}
}

func Pause(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.Pause, err = time.ParseDuration(s)
		return err
	}
}

func Stream(s string) OptionFunc {
	return func(c *Config) error {
		c.Stream = s == ""
		return nil
	}
}

func NumErrors(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.NumErrors, err = strconv.Atoi(s)
		return err
	}
}

func NumTransactions(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.NumTransactions, err = strconv.Atoi(s)
		return err
	}
}

func NumSpans(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.NumSpans, err = strconv.Atoi(s)
		return err
	}
}

func NumFrames(s string) OptionFunc {
	return func(c *Config) error {
		var err error
		c.NumFrames, err = strconv.Atoi(s)
		return err
	}
}

// Returns a runnable that simulates APM agents sending requests to APM Server with the `target` configuration
// Mutates t.Body (for compression) and t.Headers
func (t *Target) GetWork() *requester.Work {

	// Use the defaultUserAgent unless the Header contains one, which may be blank to not send the header.
	if _, ok := t.Config.Header["User-Agent"]; !ok {
		t.Config.Header.Add("User-Agent", defaultUserAgent)
	}

	if len(t.Body) > 0 {
		t.Config.Header.Add("Content-Type", "application/x-ndjson")
	}

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
		t.Config.Header.Add("Content-Encoding", "gzip")
	}

	var workReq requester.Req
	if t.Config.Stream {
		workReq = &requester.StreamReq{
			Method:        t.Method,
			Url:           t.Url,
			Header:        t.Config.Header,
			Timeout:       t.Config.RequestTimeout,
			RunTimeout:    t.Config.RunTimeout,
			EPS:           t.Config.Throttle,
			PauseDuration: t.Config.Pause,
			RequestBody:   t.Body,
		}
	} else {
		workReq = &requester.SimpleReq{
			Request:     request(t.Method, t.Url, t.Config.Header, t.Body),
			RequestBody: t.Body,
			Timeout:     int(t.Config.RequestTimeout.Seconds()),
			QPS:         t.Config.Throttle,
		}
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
		Writer:             ioutil.Discard,
	}
}

func request(method, url string, headers http.Header, body []byte) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	for header, values := range headers {
		for _, v := range values {
			req.Header.Add(header, v)
		}
	}
	req.ContentLength = int64(len(body))
	return req
}
