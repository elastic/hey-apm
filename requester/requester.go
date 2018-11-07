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
	"compress/gzip"
	"encoding/hex"
	"math/rand"
	"encoding/json"
	"container/ring"
	"bufio"
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
	flushesPerReq int64
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

// use same values as agents
const gzipBufferSize = 16000
const maxRequestWrite = 768000
const flushTimeout = time.Duration(3 * time.Second)

func (r *StreamReq) makeRequest(ctx context.Context, throttle <-chan time.Time, flushC chan int64, i int) *http.Request {
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

	ctxT, _ := context.WithTimeout(ctx, r.Timeout)
	ctx, cancel := context.WithCancel(ctx)
	req.WithContext(ctx)

	wrote := 0
	flushTimeout := time.Tick(flushTimeout)
	go func(w io.WriteCloser) {
		defer w.Close()
		var flushes int64
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		// used to cycle indefinitely through events in the request body
		ring := makeRing(r.RequestBody)

		// returns true if the request was cancelled and the caller should return
		var doFlush = func(w io.WriteCloser) bool {
			var pW = w
			// avoid last line break??
			// repeat := ring.Value.(map[string]interface{})
			// line, _ := json.Marshal(repeat)
			// gz.Write(line)
			// need this?
			gz.Flush()
			bs := buf.Bytes()
			if n, err := pW.Write(bs); err != nil {
				fmt.Println("[debug] error writing to pipe")
				flushC <- flushes
				cancel()
				return true
			} else {
				wrote += n
				flushes += 1
				if wrote > maxRequestWrite {
					cancel()
					return true
				}
			}
			return false
		}

		var lastId, lastTransactionId string
		var sentMeta bool
		for {
			select {
			case <-ctxT.Done():
				doFlush(w)
				cancel()
				flushC <- flushes
				return
			case <-flushTimeout:
				if doFlush(w) {
					return
				}
			default:
				wrapped := ring.Value.(map[string]interface{})
				ring = ring.Next()
				var reWrapped map[string]interface{}
				// randomize events so that there are no duplicated ids it creates a cascade of transactions and spans linked together
				if event, ok := wrapped["transaction"]; ok {
					transaction := event.(map[string]interface{})
					id := randHexString(8)
					transaction["id"] = id
					lastId = id
					lastTransactionId = id
					transaction["trace_id"] = randHexString(16)
					transaction["name"] = fmt.Sprintf("%s-%d", transaction["name"].(string), i)
					reWrapped = map[string]interface{}{"transaction": transaction}
				} else if event, ok := wrapped["span"]; ok {
					span := event.(map[string]interface{})
					span["parent_id"] = lastId
					span["transaction_id"] = lastTransactionId
					id := randHexString(8)
					span["id"] = id
					lastId = id
					span["trace_id"] = randHexString(16)
					span["name"] = fmt.Sprintf("%s-%d", span["name"], i)
					reWrapped = map[string]interface{}{"span": span}
				} else if event, ok := wrapped["error"]; ok {
					errEvent := event.(map[string]interface{})
					errEvent["id"] = randHexString(16)
					errEvent["trace_id"] = randHexString(16)
					errEvent["transaction_id"] = randHexString(8)
					reWrapped = map[string]interface{}{"error": errEvent}
				} else if _, ok := wrapped["metadata"]; ok {
					if sentMeta {
						// only
						continue
					} else {
						sentMeta = true
						reWrapped = wrapped
					}
				}

				line, _ := json.Marshal(reWrapped)
				// is this right?
				line = append(line, '\n')
				_, err := gz.Write(line)
				if err != nil {
					panic(err)
				}

				if buf.Len() > gzipBufferSize {
					if doFlush(w) {
						return
					}
				}

				if r.qps() > 0 {
					<-throttle
				} else {
					time.Sleep(r.PauseDuration)
				}
			}
		}

	}(pWriter)

	return req
}

func randHexString(n int) string {
	// seed needs to be initialized elsewhere
	buf := make([]byte, n)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func makeRing(b []byte) *ring.Ring {
	data := make([]map[string]interface{}, 0)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		var line map[string]interface{}
		bytes := scanner.Bytes()
		if err := json.Unmarshal(bytes, &line); err != nil {
			panic(err)
		}
		data = append(data, line)
	}
	r := ring.New(len(data))
	for _, line := range data {
		r.Value = line
		r = r.Next()
	}
	return r
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

func (r *SimpleReq) makeRequest(ctx context.Context, throttle <-chan time.Time, flushC chan int64, _ int) *http.Request {
	if r.QPS > 0 {
		<-throttle
	}
	flushC <- 1
	return cloneRequest(r.Request, r.RequestBody)
}

func (r *SimpleReq) clientTimeout() time.Duration {
	return time.Duration(r.Timeout) * time.Second
}

func (r *SimpleReq) qps() float64 {
	return r.QPS
}

type Req interface {
	makeRequest(context.Context, <-chan time.Time, chan int64, int) *http.Request
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
	rand.Seed(time.Now().UnixNano())
	b.results = make(chan *result, min(b.C*1000, maxResult))
	b.stopCh = make(chan struct{}, b.C)
	b.start = time.Now()
	b.report = newReport(b.writer(), b.results, b.Output, b.N)
	// Run the reporter first, it polls the result channel until it is closed.
	go func() {
		runReporter(b.report)
	}()

	b.runWorkers()
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

func (b *Work) sendReq(ctx context.Context, client *http.Client, throttle <-chan time.Time, i int) {
	var size int64
	var code int
	var dnsStart, connStart, resStart, reqStart, delayStart time.Time
	var dnsDuration, connDuration, reqDuration, delayDuration, resDuration time.Duration

	flushC := make(chan int64, 1)
	req := b.Req.makeRequest(ctx, throttle, flushC, i)

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
	t := time.Now()
	resDuration = t.Sub(resStart)
	finish := t.Sub(st)
	flushes := <-flushC
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
		flushesPerReq: flushes,
	}
}

func (b *Work) runWorker(client *http.Client, n int) {
	var throttle <-chan time.Time
	if b.Req.qps() > 0 {
		throttle = time.Tick(time.Duration(1e6/(b.Req.qps())) * time.Microsecond)
	}

	if b.DisableRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), b.Req.clientTimeout())
	for i := 0; i < n; i++ {
		// Check if application is stopped. Do not send into a closed channel.
		select {
		case <-b.stopCh:
			cancel()
			return
		default:
			b.sendReq(ctx, client, throttle, i)
		}
	}
}

func (b *Work) runWorkers() {
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
			b.runWorker(client, b.N/b.C)
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

func (b *Work) ErrorDist() map[string]int {
	return b.report.errorDist
}

func (b *Work) StatusCodes() map[int]int {
	return b.report.statusCodeDist
}

func (b *Work) Flushes() int64 {
	return b.report.flushesTotal
}
