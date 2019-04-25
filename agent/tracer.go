package agent

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/strcoll"

	"go.elastic.co/apm"
	apmtransport "go.elastic.co/apm/transport"
)

type Tracer struct {
	*apm.Tracer
	TransportStats *transportStats
}

type transportStats struct {
	Accepted  float64
	TopErrors []string
}

func (t Tracer) Close() {
	t.Tracer.Close()
	t.Transport.(*transport).wg.Wait()
	close(t.Transport.(*transport).responses)
}

func NewTracer(logger apm.Logger, serverUrl, serverSecret string, maxSpans int) *Tracer {

	goTracer := apm.DefaultTracer
	transport := wrap(goTracer.Transport.(*apmtransport.HTTPTransport), serverUrl, serverSecret)
	goTracer.Transport = transport
	goTracer.SetLogger(logger)
	goTracer.SetMetricsInterval(0) // disable metrics
	goTracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	goTracer.SetMaxSpans(maxSpans)

	tracer := &Tracer{goTracer, &transportStats{}}

	go func() {
		for {
			select {
			case response := <-transport.responses:
				var m map[string]interface{}
				if err := json.Unmarshal(response, &m); err != nil {
					return
				}
				tracer.TransportStats.Accepted += conv.AsFloat64(m, "accepted")
				for _, i := range conv.AsSlice(m, "errors") {
					e := conv.AsString(i, "message")
					if !strcoll.Contains(e, tracer.TransportStats.TopErrors) {
						tracer.TransportStats.TopErrors = append(tracer.TransportStats.TopErrors, e)
					}
				}
				transport.wg.Done()
			}
		}
	}()
	return tracer
}

type transport struct {
	*apmtransport.HTTPTransport
	headers   http.Header
	url       *url.URL
	responses chan []byte
	wg        sync.WaitGroup
}

func wrap(backend *apmtransport.HTTPTransport, serverUrl, serverSecret string) *transport {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/x-ndjson")
	headers.Set("Content-Encoding", "deflate")
	headers.Set("Transfer-Encoding", "chunked")
	headers.Set("User-Agent", "hey-apm")
	headers.Set("Accept", "application/json")
	if serverSecret != "" {
		headers.Set("Authorization", "Bearer "+serverSecret)
	}
	u, err := url.Parse(serverUrl + "/intake/v2/events?verbose")
	if err != nil {
		panic(err)
	}
	return &transport{HTTPTransport: backend, headers: headers, url: u, responses: make(chan []byte, 0)}
}

func (t *transport) SendStream(ctx context.Context, r io.Reader) error {
	req := t.newRequest(t.url)
	req = requestWithContext(ctx, req)
	req.Body = ioutil.NopCloser(r)
	if err := t.sendRequest(req); err != nil {
		return err
	}
	return nil
}

func (t *transport) sendRequest(req *http.Request) error {
	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyContents, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		t.responses <- bodyContents
		t.wg.Add(1)
	}
	return err
}

func (t *transport) newRequest(url *url.URL) *http.Request {
	req := &http.Request{
		Method:     "POST",
		URL:        url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     t.headers,
		Host:       url.Host,
	}
	return req
}

func requestWithContext(ctx context.Context, req *http.Request) *http.Request {
	url := req.URL
	req.URL = nil
	reqCopy := req.WithContext(ctx)
	reqCopy.URL = url
	req.URL = url
	return reqCopy
}
