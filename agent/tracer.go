package agent

import (
	"bytes"
	"encoding/json"
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
	TransportStats *TransportStats
}

// TransportStats are captured by reading apm-server responses.
type TransportStats struct {
	Accepted    uint64
	TopErrors   []string
	NumRequests uint64
}

func (t Tracer) Close() {
	t.Tracer.Close()
	rt := t.Transport.(*apmtransport.HTTPTransport).Client.Transport.(*roundTripper)
	rt.wg.Wait()
	close(rt.c)
}

// NewTracer returns a wrapper with a new Go agent instance and its transport stats.
func NewTracer(logger apm.Logger, serverUrl, serverSecret, serviceName string, maxSpans int) *Tracer {
	// version can be set with ELASTIC_APM_SERVICE_VERSION
	goTracer, _ := apm.NewTracer(serviceName, "")
	goTracer.SetLogger(logger)
	goTracer.SetMetricsInterval(0) // disable metrics
	goTracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	goTracer.SetMaxSpans(maxSpans)

	transport := goTracer.Transport.(*apmtransport.HTTPTransport)
	transport.SetUserAgent("hey-apm")
	if serverSecret != "" {
		transport.SetSecretToken(serverSecret)
	}
	if serverUrl != "" {
		u, err := url.Parse(serverUrl)
		if err != nil {
			panic(err)
		}
		transport.SetServerURL(u)
	}
	rt := &roundTripper{c: make(chan []byte, 0)}
	transport.Client.Transport = rt

	tracer := &Tracer{goTracer, &TransportStats{}}

	// TODO confirm that synchronization is wired up correctly
	go func() {
		for response := range rt.c {
			var m map[string]interface{}
			if err := json.Unmarshal(response, &m); err != nil {
				return
			}
			tracer.TransportStats.Accepted += conv.AsUint64(m, "accepted")
			tracer.TransportStats.NumRequests += 1
			for _, i := range conv.AsSlice(m, "errors") {
				e := conv.AsString(i, "message")
				if !strcoll.Contains(e, tracer.TransportStats.TopErrors) {
					tracer.TransportStats.TopErrors = append(tracer.TransportStats.TopErrors, e)
				}
			}
			rt.wg.Done()
		}
	}()
	return tracer
}

type roundTripper struct {
	c  chan []byte
	wg sync.WaitGroup
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Path {
	case "/intake/v2/events", "/intake/v2/rum/events":
	default:
		return http.DefaultTransport.RoundTrip(req)
	}

	q := req.URL.Query()
	q.Set("verbose", "")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()

	if resp.Body == http.NoBody {
		return resp, err
	}

	b, rerr := ioutil.ReadAll(resp.Body)
	if rerr == nil {
		rt.wg.Add(1)
		rt.c <- b
		resp.Body = ioutil.NopCloser(bytes.NewReader(b))
	}

	return resp, err
}
