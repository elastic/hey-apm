package agent

import (
	"net/url"
	"time"

	"go.elastic.co/apm"
	apmtransport "go.elastic.co/apm/transport"
)

func Tracer(logger apm.Logger, serverUrl, serverSecret string, maxSpans int) *apm.Tracer {
	tracer := apm.DefaultTracer
	tracer.SetLogger(logger)
	tracer.SetMetricsInterval(0) // disable metrics
	transport := tracer.Transport.(*apmtransport.HTTPTransport)
	transport.SetUserAgent("hey-apm")
	if serverSecret != "" {
		transport.SetSecretToken(serverSecret)
	}
	if serverUrl != "" {
		u, err := url.Parse(serverUrl)
		if err != nil {
			logger.Errorf("invalid apm server url:", err)
		}
		transport.SetServerURL(u)
	}

	tracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	tracer.SetMaxSpans(maxSpans)
	return tracer
}
