package main

import (
	"flag"
	"log"
	"math"
	"net/url"
	"os"
	"time"

	"go.elastic.co/apm"
	apmtransport "go.elastic.co/apm/transport"
)

func main() {
	// run options
	runTimeout := flag.Duration("run", 30*time.Second, "stop run after this duration")

	// apm-server options
	apmServerSecret := flag.String("secret", "", "") // ELASTIC_APM_SECRET_TOKEN
	apmServerUrl := flag.String("url", "", "")       // ELASTIC_APM_SERVER_URL

	// payload options
	//errorLimit := flag.Int("e", math.MaxInt64, "max errors to generate")
	spanMaxLimit := flag.Int("sx", 10, "max spans to per transaction")
	spanMinLimit := flag.Int("sm", 1, "min spans to per transaction")
	transactionLimit := flag.Int("t", math.MaxInt64, "max transactions to generate")
	flag.Parse()

	if *spanMaxLimit < *spanMinLimit {
		spanMaxLimit = spanMinLimit
	}

	// configure tracer
	logger := newApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))

	tracer := apm.DefaultTracer
	defer func() {
		flushed := make(chan struct{})
		go func() {
			tracer.Flush(nil)
			close(flushed)
		}()

		select {
		case <-flushed:
		case <-time.After(10 * time.Second):
			// give up waiting for flush
		}
		tracer.Close()
	}()
	tracer.SetLogger(logger)
	tracer.SetMetricsInterval(0) // disable metrics
	transport := tracer.Transport.(*apmtransport.HTTPTransport)
	transport.SetUserAgent("hey-apm")
	if *apmServerSecret != "" {
		transport.SetSecretToken(*apmServerSecret)
	}
	if *apmServerUrl != "" {
		u, err := url.Parse(*apmServerUrl)
		if err != nil {
			logger.Fatal("invalid apm server url:", err)
		}
		transport.SetServerURL(u)
	}

	//tracer.SetSpanFramesMinDuration(0)
	tracer.SetMaxSpans(*spanMaxLimit)

	w := worker{
		apmLogger:  logger,
		Tracer:     tracer,
		runTimeout: *runTimeout,
	}
	w.addTransactions(*transactionLimit, *spanMinLimit, *spanMaxLimit)
	report, _ := w.Work()
	logger.Printf("complete after %d events in %s", report.Count, report.End.Sub(report.Start))
}
