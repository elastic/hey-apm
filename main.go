package main

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"time"

	"go.elastic.co/apm"
	apmtransport "go.elastic.co/apm/transport"
)

func main() {
	// run options
	runTimeout := flag.Duration("run", 30*time.Second, "stop run after this duration")
	flushTimeout := flag.Duration("flush", 10*time.Second, "wait timeout for agent flush")
	seed := flag.Int64("seed", time.Now().Unix(), "random seed")

	// apm-server options
	// convenience for https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html
	apmServerSecret := flag.String("secret", "", "") // ELASTIC_APM_SECRET_TOKEN
	apmServerUrl := flag.String("url", "", "")       // ELASTIC_APM_SERVER_URL

	// payload options
	errorLimit := flag.Int("e", math.MaxInt64, "max errors to generate")
	errorFrequency := flag.Duration("ef", 1*time.Nanosecond, "error frequency. "+
		"generate errors up to once in this duration")
	errorFrameMaxLimit := flag.Int("ex", 10, "max error frames to per error")
	errorFrameMinLimit := flag.Int("em", 0, "max error frames to per error")
	spanMaxLimit := flag.Int("sx", 10, "max spans to per transaction")
	spanMinLimit := flag.Int("sm", 1, "min spans to per transaction")
	transactionLimit := flag.Int("t", math.MaxInt64, "max transactions to generate")
	transactionFrequency := flag.Duration("tf", 1*time.Nanosecond, "transaction frequency. "+
		"generate transactions up to once in this duration")
	flag.Parse()

	if *spanMaxLimit < *spanMinLimit {
		spanMaxLimit = spanMinLimit
	}

	// configure tracer
	logger := newApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	rand.Seed(*seed)
	logger.Debugf("random seed: %d", *seed)

	tracer := apm.DefaultTracer
	flush := func() {
		flushed := make(chan struct{})
		go func() {
			tracer.Flush(nil)
			close(flushed)
		}()

		flushWait := time.After(*flushTimeout)
		if *flushTimeout == 0 {
			flushWait = make(<-chan time.Time)
		}
		select {
		case <-flushed:
		case <-flushWait:
			// give up waiting for flush
			logger.Errorf("timed out waiting for flush to complete")
		}
		tracer.Close()
	}
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

	tracer.SetSpanFramesMinDuration(1 * time.Nanosecond)
	tracer.SetMaxSpans(*spanMaxLimit)

	w := worker{
		apmLogger:  logger,
		Tracer:     tracer,
		runTimeout: *runTimeout,
	}
	w.addErrors(throttle(time.NewTicker(*errorFrequency).C), *errorLimit, *errorFrameMinLimit, *errorFrameMaxLimit)
	w.addTransactions(throttle(time.NewTicker(*transactionFrequency).C), *transactionLimit, *spanMinLimit, *spanMaxLimit)

	logger.Debugf("start")
	defer logger.Debugf("finish")
	report, _ := w.Work()
	defer func() { logger.Debugf("%s elapsed since event generation completed", time.Now().Sub(report.End)) }()
	defer flush()
	e, t, s := w.Counts()
	logger.Printf("generated %d events (errors: %d, transctions: %d, spans: %d) in %s",
		e+t+s, e, t, s, report.End.Sub(report.Start))
	if report.Count != t {
		logger.Errorf("unexpected sampling decision count, expected: %d got: %d", t, report.Count)
	}
}

// throttle converts a time ticker to a channel of things
func throttle(c <-chan time.Time) chan interface{} {
	throttle := make(chan interface{})
	go func() {
		for range c {
			throttle <- struct{}{}
		}
	}()
	return throttle
}
