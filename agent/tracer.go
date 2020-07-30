package agent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.elastic.co/apm"
	apmtransport "go.elastic.co/apm/transport"
)

type Tracer struct {
	*apm.Tracer
	roundTripper *roundTripperWrapper
}

func (t *Tracer) TransportStats() TransportStats {
	t.roundTripper.statsMu.RLock()
	defer t.roundTripper.statsMu.RUnlock()
	return t.roundTripper.stats
}

// TransportStats are captured by reading apm-server responses.
type TransportStats struct {
	EventsAccepted uint64
	UniqueErrors   []string
	NumRequests    uint64
}

// NewTracer returns a wrapper with a new Go agent instance and its transport stats.
func NewTracer(
	logger apm.Logger,
	serverURL, serverSecret, apiKey, serviceName string,
	maxSpans int,
) (*Tracer, error) {

	// Ensure that each tracer uses an independent transport.
	transport, err := apmtransport.NewHTTPTransport()
	if err != nil {
		return nil, err
	}
	transport.SetUserAgent("hey-apm")
	if apiKey != "" {
		transport.SetAPIKey(apiKey)
	} else if serverSecret != "" {
		transport.SetSecretToken(serverSecret)
	}
	if serverURL != "" {
		u, err := url.Parse(serverURL)
		if err != nil {
			panic(err)
		}
		transport.SetServerURL(u)
	}
	roundTripper := &roundTripperWrapper{
		roundTripper: transport.Client.Transport,
		logger:       logger,
		uniqueErrors: make(map[string]struct{}),
	}
	transport.Client.Transport = roundTripper

	goTracer, err := apm.NewTracerOptions(apm.TracerOptions{
		ServiceName: serviceName,
		Transport:   transport,
	})
	if err != nil {
		return nil, err
	}
	goTracer.SetLogger(logger)
	goTracer.SetMetricsInterval(0) // disable metrics
	goTracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	goTracer.SetMaxSpans(maxSpans)
	return &Tracer{Tracer: goTracer, roundTripper: roundTripper}, nil
}

type roundTripperWrapper struct {
	roundTripper http.RoundTripper
	logger       apm.Logger

	statsMu      sync.RWMutex
	stats        TransportStats
	uniqueErrors map[string]struct{}
}

func (rt *roundTripperWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Path {
	case "/intake/v2/events", "/intake/v2/rum/events":
	default:
		return rt.roundTripper.RoundTrip(req)
	}

	q := req.URL.Query()
	q.Set("verbose", "")
	req.URL.RawQuery = q.Encode()

	resp, err := rt.roundTripper.RoundTrip(req)
	if err != nil {
		// Number of *failed* requests is tracked by the Go Agent.
		rt.statsMu.Lock()
		rt.stats.NumRequests++
		rt.statsMu.Unlock()
		return resp, err
	}
	defer resp.Body.Close()

	rt.statsMu.Lock()
	defer rt.statsMu.Unlock()
	rt.stats.NumRequests++

	if resp.Body != http.NoBody {
		if data, rerr := ioutil.ReadAll(resp.Body); rerr == nil {
			resp.Body = ioutil.NopCloser(bytes.NewReader(data))

			var response intakeResponse
			if err := json.Unmarshal(data, &response); err != nil {
				rt.logger.Errorf("failed to decode response: %s", err)
			} else {
				rt.stats.EventsAccepted += response.Accepted
				for _, e := range response.Errors {
					if _, ok := rt.uniqueErrors[e.Message]; !ok {
						rt.uniqueErrors[e.Message] = struct{}{}
						rt.stats.UniqueErrors = append(rt.stats.UniqueErrors, e.Message)
					}
				}
			}
		}
	}
	return resp, err
}

type intakeResponse struct {
	Accepted uint64
	Errors   []struct {
		Message  string
		Document string
	}
}
