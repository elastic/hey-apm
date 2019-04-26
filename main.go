package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/worker"

	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/server"
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
	logger := out.NewApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	rand.Seed(*seed)
	logger.Debugf("random seed: %d", *seed)

	tracer := agent.NewTracer(logger, *apmServerUrl, *apmServerSecret, *spanMaxLimit)

	w := worker.Worker{
		ApmLogger:    logger,
		Tracer:       tracer,
		RunTimeout:   *runTimeout,
		FlushTimeout: *flushTimeout,
	}
	w.AddErrors(*errorFrequency, *errorLimit, *errorFrameMinLimit, *errorFrameMaxLimit)
	w.AddTransactions(*transactionFrequency, *transactionLimit, *spanMinLimit, *spanMaxLimit)

	logger.Debugf("start")
	defer logger.Debugf("finish")

	metricsBefore, merr1 := server.QueryExpvar(*apmServerSecret, *apmServerUrl)

	report, err := w.Work()
	if err != nil {
		logger.Errorf(err.Error())
	}
	logger.Debugf("%s elapsed since event generation completed", report.Flushed.Sub(report.End))

	fmt.Println()
	fmt.Println(report.Stats.Format(30))

	metricsAfter, merr2 := server.QueryExpvar(*apmServerSecret, *apmServerUrl)
	if merr2 == nil && merr1 == nil {
		fmt.Println()
		fmt.Println(metricsAfter.Memstats.Sub(metricsBefore.Memstats))
	}

	info, err := server.QueryInfo(*apmServerSecret, *apmServerUrl)
	if err != nil {
		logger.Errorf("apm-server health error: %s", err.Error())
	} else {
		fmt.Println(out.Bold + "\n*** " + info.String() + " ***\n" + out.Reset)
	}
}
