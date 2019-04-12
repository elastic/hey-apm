package main

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/tracer"
	"github.com/elastic/hey-apm/work"
)

func main() {
	// run options
	runTimeout := flag.Duration("run", 30*time.Second, "stop run after this duration")
	flushTimeout := flag.Duration("flush", 10*time.Second, "wait timeout for agent flush")
	seed := flag.Int64("seed", time.Now().Unix(), "random seed")

	// apm-server options
	// convenience for https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html
	apmServerSecret := flag.String("secret", "", "")                // ELASTIC_APM_SECRET_TOKEN
	apmServerUrl := flag.String("url", "http://localhost:8200", "") // ELASTIC_APM_SERVER_URL

	// payload options
	errorLimit := flag.Int("e", math.MaxInt64, "max errors to generate")
	errorFrequency := flag.Duration("ef", 1*time.Nanosecond, "error frequency. "+
		"generate errors up to once in this duration")
	errorFrameMaxLimit := flag.Int("ex", 10, "max error frames per error")
	errorFrameMinLimit := flag.Int("em", 1, "min error frames per error")
	transactionLimit := flag.Int("t", math.MaxInt64, "max transactions to generate")
	transactionFrequency := flag.Duration("tf", 1*time.Nanosecond, "transaction frequency. "+
		"generate transactions up to once in this duration")
	spanMaxLimit := flag.Int("sx", 10, "max spans per transaction")
	spanMinLimit := flag.Int("sm", 1, "min spans per transaction")

	flag.Parse()

	if *spanMaxLimit < *spanMinLimit {
		spanMaxLimit = spanMinLimit
	}
	if *errorFrameMaxLimit < *errorFrameMinLimit {
		errorFrameMaxLimit = errorFrameMinLimit
	}

	logger := out.NewApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	rand.Seed(*seed)

	w := work.Worker{
		RunTimeout:             *runTimeout,
		TransactionLimit:       *transactionLimit,
		TransactionFrequency:   *transactionFrequency,
		MaxSpansPerTransaction: *spanMaxLimit,
		MinSpansPerTransaction: *spanMinLimit,
		ErrorLimit:             *errorLimit,
		ErrorFrequency:         *errorFrequency,
		MaxFramesPerError:      *errorFrameMaxLimit,
		MinFramesPerError:      *errorFrameMinLimit,
	}
	w.Add(work.HandleSignals())

	logger.Debugf("start")
	defer logger.Debugf("finish")
	tracer := tracer.NewTracer(logger, *flushTimeout, *apmServerSecret, *apmServerUrl)

	report, err := w.Work(tracer)
	if err != nil {
		logger.Errorf(err.Error())
	}
	logger.Debugf("%s elapsed since event generation completed", time.Now().Sub(report.Stop))
	e, de := report.Stats.ErrorsSent, report.Stats.ErrorsDropped
	t, dt := report.Stats.TransactionsSent, report.Stats.TransactionsDropped
	s, ds := report.Stats.SpansSent, report.Stats.SpansDropped
	logger.Printf("sent %d events in %.1f seconds (%d dropped)", e+t+s, report.End.Sub(report.Start).Seconds(), de+dt+ds)
	logger.Printf("    transactions (sent / dropped) : %d / %d [%.2f%%] ", t, dt, per(t, dt))
	logger.Printf("    spans (sent / dropped)        : %d / %d [%.2f%%] ", s, ds, per(s, ds))
	logger.Printf("    errors (sent / dropped)       : %d / %d [%.2f%%] ", e, de, per(e, de))
	logger.Printf("%d request errors", report.Stats.Errors.SendStream)
}

func per(i1, i2 uint64) float64 {
	return float64(i1) * 100 / (float64(i1) + float64(i2))
}
