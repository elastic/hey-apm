package tracer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	apmtransport "go.elastic.co/apm/transport"

	"go.elastic.co/apm"
)

type Tracer struct {
	*apm.Tracer
	logger  apm.Logger
	timeout time.Duration
}

func NewTracer(logger apm.Logger, timeout time.Duration, serverSecret, serverUrl string) *Tracer {
	tracer := apm.DefaultTracer
	transport := tracer.Transport.(*apmtransport.HTTPTransport)
	tracer.Transport = wrap(transport, logger, serverSecret, serverUrl)
	tracer.SetLogger(logger)
	tracer.SetMetricsInterval(0) // disable metrics
	tracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	return &Tracer{tracer, logger, timeout}
}

func (t *Tracer) FlushAll() {
	flushed := make(chan struct{})
	go func() {
		t.Flush(nil)
		close(flushed)
	}()

	flushWait := time.After(t.timeout)
	if t.timeout == 0 {
		flushWait = make(<-chan time.Time)
	}
	select {
	case <-flushed:
	case <-flushWait:
		// give up waiting for flush
		t.logger.Errorf("timed out waiting for flush to complete")
	}
	t.Close()
}

type transport struct {
	*apmtransport.HTTPTransport
	headers http.Header
	url     *url.URL
	logger  apm.Logger
}

func wrap(backend *apmtransport.HTTPTransport, logger apm.Logger, serverSecret, serverUrl string) *transport {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/x-ndjson")
	headers.Set("Content-Encoding", "deflate")
	headers.Set("Transfer-Encoding", "chunked")
	headers.Set("User-Agent", "hey-apm")
	headers.Set("Accept", "application/json")
	if serverSecret != "" {
		headers.Set("Authorization", "Bearer "+serverSecret)
	}
	u, err := url.Parse(serverUrl)
	u.Path = "/intake/v2/events"
	if err != nil {
		panic(err)
	}
	return &transport{backend, headers, u, logger}
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
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		resp.Body.Close()
		return nil
	}
	defer resp.Body.Close()

	bodyContents, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		resp.Body = ioutil.NopCloser(bytes.NewReader(bodyContents))
	}
	return nil
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
