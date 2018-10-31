// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package requester provides commands to run load tests and display results.
package requester

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// Max size of the buffer of result channel.
const maxResult = 1000000
const maxIdleConn = 500

type result struct {
	err           error
	statusCode    int
	duration      time.Duration
	connDuration  time.Duration // connection setup(DNS lookup + Dial up) duration
	dnsDuration   time.Duration // dns lookup duration
	reqDuration   time.Duration // request "write" duration
	resDuration   time.Duration // response "read" duration
	delayDuration time.Duration // delay between response and request
	contentLength int64
}

type StreamReq struct {
	Method, Url string
	RequestBody []byte
	Header      http.Header
	Username    string
	Password    string

	// Timeout per request in duration.
	Timeout time.Duration

	// RunTimeout in duration.
	RunTimeout time.Duration

	// Pause between streaming events. Ignored if EPS is set.
	PauseDuration time.Duration

	// EPS is the rate limit in events per second.
	EPS float64
}

func (r *StreamReq) makeRequest(ctx context.Context, throttle <-chan time.Time) (*http.Request, context.CancelFunc) {
	pReader, pWriter := io.Pipe()
	req, err := http.NewRequest(r.Method, r.Url, pReader)
	if err != nil {
		panic(err)
	}
	//set auth
	if r.Username != "" || r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}

	// deep copy of the Header
	req.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		req.Header[k] = append([]string(nil), s...)
	}

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)

	go func(w io.WriteCloser) {
		defer w.Close()
		var pW = w

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if _, err := pW.Write(r.RequestBody); err != nil {
					fmt.Println("[debug] error writing to pipe")
					return
				}
				if r.qps() > 0 {
					<-throttle
				} else {
					time.Sleep(r.PauseDuration)
				}
			}
		}
	}(pWriter)
	return req, cancel
}

func (r *StreamReq) ctxRun() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.RunTimeout)
}
func (r *StreamReq) clientTimeout() time.Duration {
	return r.RunTimeout
}
func (r *StreamReq) qps() float64 {
	return r.EPS
}

type SimpleReq struct {
	// Request is the request to be made.
	Request     *http.Request
	RequestBody []byte

	// Request Timeout in seconds.
	Timeout int

	// Qps is the rate limit in queries per second.
	QPS float64
}

func (r *SimpleReq) makeRequest(ctx context.Context, throttle <-chan time.Time) (*http.Request, context.CancelFunc) {
	if r.QPS > 0 {
		<-throttle
	}
	return cloneRequest(r.Request, r.RequestBody), func() {}
}

func (r *SimpleReq) ctxRun() (context.Context, context.CancelFunc) {
	return context.TODO(), func() {}
}

func (r *SimpleReq) clientTimeout() time.Duration {
	return time.Duration(r.Timeout) * time.Second
}

func (r *SimpleReq) qps() float64 {
	return r.QPS
}

type Req interface {
	makeRequest(context.Context, <-chan time.Time) (*http.Request, context.CancelFunc)
	ctxRun() (context.Context, context.CancelFunc)
	clientTimeout() time.Duration
	qps() float64
}

type Work struct {
	Req Req

	// N is the total number of requests to make.
	N int

	// C is the concurrency level, the number of concurrent workers to run.
	C int

	// H2 is an option to make HTTP/2 requests
	H2 bool

	// DisableCompression is an option to disable compression in response
	DisableCompression bool

	// DisableKeepAlives is an option to prevents re-use of TCP connections between different HTTP requests
	DisableKeepAlives bool

	// DisableRedirects is an option to prevent the following of HTTP redirects
	DisableRedirects bool

	// Output represents the output type. If "csv" is provided, the
	// output will be dumped as a csv stream.
	Output string

	// ProxyAddr is the address of HTTP proxy server in the format on "host:port".
	// Optional.
	ProxyAddr *url.URL

	// Writer is where results will be written. If nil, results are written to stdout.
	Writer io.Writer

	results chan *result
	stopCh  chan struct{}
	start   time.Time

	report *report
}

func (b *Work) writer() io.Writer {
	if b.Writer == nil {
		return os.Stdout
	}
	return b.Writer
}

// Run makes all the requests, prints the summary. It blocks until
// all work is done.
func (b *Work) Run() {
	b.results = make(chan *result, min(b.C*1000, maxResult))
	b.stopCh = make(chan struct{}, b.C)
	b.start = time.Now()
	b.report = newReport(b.writer(), b.results, b.Output, b.N)
	// Run the reporter first, it polls the result channel until it is closed.
	go func() {
		runReporter(b.report)
	}()

	ctx, cancel := b.Req.ctxRun()
	b.runWorkers(ctx)
	cancel()
	b.Finish()
}

func (b *Work) Stop() {
	// Send stop signal so that workers can stop gracefully.
	for i := 0; i < b.C; i++ {
		b.stopCh <- struct{}{}
	}
}

func (b *Work) Finish() {
	close(b.results)
	total := time.Now().Sub(b.start)
	// Wait until the reporter is done.
	<-b.report.done
	b.report.finalize(total)
}

func (b *Work) sendReq(ctx context.Context, client *http.Client, throttle <-chan time.Time) {
	var size int64
	var code int
	var dnsStart, connStart, resStart, reqStart, delayStart time.Time
	var dnsDuration, connDuration, reqDuration, delayDuration, resDuration time.Duration

	req, cancel := b.Req.makeRequest(ctx, throttle)

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			dnsDuration = time.Now().Sub(dnsStart)
		},
		GetConn: func(h string) {
			connStart = time.Now()
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if !connInfo.Reused {
				connDuration = time.Now().Sub(connStart)
			}
			reqStart = time.Now()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			reqDuration = time.Now().Sub(reqStart)
			delayStart = time.Now()
		},
		GotFirstResponseByte: func() {
			delayDuration = time.Now().Sub(delayStart)
			resStart = time.Now()
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	st := time.Now()
	resp, err := client.Do(req)
	if err == nil {
		size = resp.ContentLength
		code = resp.StatusCode
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	cancel()
	t := time.Now()
	resDuration = t.Sub(resStart)
	finish := t.Sub(st)
	b.results <- &result{
		statusCode:    code,
		duration:      finish,
		err:           err,
		contentLength: size,
		connDuration:  connDuration,
		dnsDuration:   dnsDuration,
		reqDuration:   reqDuration,
		resDuration:   resDuration,
		delayDuration: delayDuration,
	}

}

func (b *Work) runWorker(parent context.Context, client *http.Client, n int) {
	var throttle <-chan time.Time
	if b.Req.qps() > 0 {
		throttle = time.Tick(time.Duration(1e6/(b.Req.qps())) * time.Microsecond)
	}

	if b.DisableRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	ctx, cancel := context.WithCancel(parent)
	for i := 0; i < n; i++ {
		// Check if application is stopped. Do not send into a closed channel.
		select {
		case <-b.stopCh:
			cancel()
			return
		default:
			b.sendReq(ctx, client, throttle)
		}
	}
}

func (b *Work) runWorkers(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(b.C)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConnsPerHost: min(b.C, maxIdleConn),
		DisableCompression:  b.DisableCompression,
		DisableKeepAlives:   b.DisableKeepAlives,
		Proxy:               http.ProxyURL(b.ProxyAddr),
	}
	if b.H2 {
		http2.ConfigureTransport(tr)
	} else {
		tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	}
	client := &http.Client{Transport: tr, Timeout: b.Req.clientTimeout()}

	// Ignore the case where b.N % b.C != 0.
	for i := 0; i < b.C; i++ {
		go func() {
			b.runWorker(ctx, client, b.N/b.C)
			wg.Done()
		}()
	}
	wg.Wait()
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request, body []byte) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	if len(body) > 0 {
		r2.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	return r2
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (w *Work) ErrorDist() map[string]int {
	return w.report.errorDist
}

func (w *Work) StatusCodes() map[int]int {
	return w.report.statusCodeDist
}
