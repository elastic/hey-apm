package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/reports"
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
	apmServerSecret := flag.String("apm-secret", "", "apm server secret token")       // ELASTIC_APM_SECRET_TOKEN
	apmServerUrl := flag.String("apm-url", "http://localhost:8200", "apm server url") // ELASTIC_APM_SERVER_URL

	elasticsearchUrl := flag.String("es-url", "http://localhost:9200", "elasticsearch url for reporting")
	elasticsearchAuth := flag.String("es-auth", "", "elasticsearch username:password reporting")

	apmElasticsearchUrl := flag.String("apm-es-url", "http://localhost:9200", "elasticsearch output host for apm-server under load")
	apmElasticsearchAuth := flag.String("apm-es-auth", "", "elasticsearch output username:password for apm-server under load")

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

	rand.Seed(*seed)

	input := Input{
		ApmServerUrl:         *apmServerUrl,
		ApmServerSecret:      *apmServerSecret,
		ElasticsearchUrl:     *elasticsearchUrl,
		ElasticsearchAuth:    *elasticsearchAuth,
		ApmElasticsearchUrl:  *apmElasticsearchUrl,
		ApmElasticsearchAuth: *apmElasticsearchAuth,
		RunTimeout:           *runTimeout,
		FlushTimeout:         *flushTimeout,
		TransactionFrequency: *transactionFrequency,
		TransactionLimit:     *transactionLimit,
		SpanMaxLimit:         *spanMaxLimit,
		SpanMinLimit:         *spanMinLimit,
		ErrorFrequency:       *errorFrequency,
		ErrorLimit:           *errorLimit,
		ErrorFrameMaxLimit:   *errorFrameMaxLimit,
		ErrorFrameMinLimit:   *errorFrameMinLimit,
	}

	testNode := es.NewConnection(*elasticsearchUrl, *elasticsearchAuth)
	worker, initialStatus := prepareWork(input, testNode)
	logger := worker.Logger

	result, err := worker.Work()
	if err != nil {
		logger.Println(err.Error())
	}
	logger.Printf("%s elapsed since event generation completed", result.Flushed.Sub(result.End))
	fmt.Println(result)

	finalStatus := server.GetStatus(logger, input.ApmServerSecret, input.ApmServerUrl, testNode)
	report := createReport(logger, input, result, initialStatus, finalStatus)

	if *elasticsearchUrl == "" {
		logger.Println("es-url unset: not indexing report")

	} else {
		reportNode := es.NewConnection(*elasticsearchUrl, *elasticsearchAuth)
		if ierr := es.IndexReport(reportNode, "hey-bench", report); ierr != nil {
			logger.Println(ierr.Error())
		} else {
			logger.Println("report indexed")
		}
	}
}

type Input struct {
	ApmServerUrl    string
	ApmServerSecret string

	ElasticsearchUrl  string
	ElasticsearchAuth string

	ApmElasticsearchUrl  string
	ApmElasticsearchAuth string

	RunTimeout   time.Duration
	FlushTimeout time.Duration

	TransactionFrequency time.Duration
	TransactionLimit     int

	SpanMaxLimit int
	SpanMinLimit int

	ErrorFrequency     time.Duration
	ErrorLimit         int
	ErrorFrameMaxLimit int
	ErrorFrameMinLimit int
}

func host(urlStr string) string {
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	return url.Hostname()
}

// not UUID
func shortId() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b[0:4])
}

func prepareWork(input Input, connection es.Connection) (worker.Worker, server.Status) {

	logger := out.NewApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	tracer := agent.NewTracer(logger, input.ApmServerUrl, input.ApmServerSecret, input.SpanMaxLimit)

	w := worker.Worker{
		ApmLogger:    logger,
		Tracer:       tracer,
		RunTimeout:   input.RunTimeout,
		FlushTimeout: input.FlushTimeout,
	}
	w.AddErrors(input.ErrorFrequency, input.ErrorLimit, input.ErrorFrameMinLimit, input.ErrorFrameMaxLimit)
	w.AddTransactions(input.TransactionFrequency, input.TransactionLimit, input.SpanMinLimit, input.SpanMaxLimit)
	w.AddSignalHandling()

	return w, server.GetStatus(logger.Logger, input.ApmServerSecret, input.ApmServerUrl, connection)
}

func createReport(logger *log.Logger, input Input, result worker.Result, initialStatus, finalStatus server.Status) reports.Report {

	this, _ := os.Hostname()
	r := reports.Report{
		ReportId:                       shortId(),
		ReportDate:                     time.Now().Format(reports.GITRFC),
		ReporterHost:                   this,
		Timestamp:                      time.Now(),
		ElasticHost:                    host(input.ApmElasticsearchUrl),
		ApmHost:                        host(input.ApmServerUrl),
		NumApms:                        1,
		RunTimeout:                     input.RunTimeout.Seconds(),
		Elapsed:                        result.Flushed.Sub(result.Start).Seconds(),
		Requests:                       result.NumRequests,
		FailedRequests:                 result.Errors.SendStream,
		ErrorsGenerated:                result.ErrorsSent + result.ErrorsDropped,
		ErrorsSent:                     result.ErrorsSent,
		ErrorGenerationFrequency:       input.ErrorFrequency,
		TransactionsGenerated:          result.TransactionsSent + result.TransactionsDropped,
		TransactionsSent:               result.TransactionsSent,
		TransactionGenerationFrequency: input.TransactionFrequency,
		SpansGenerated:                 result.SpansSent + result.SpansDropped,
		SpansSent:                      result.SpansSent,
		EventsAccepted:                 result.Accepted,
	}

	info, ierr := server.QueryInfo(input.ApmServerSecret, input.ApmServerUrl)
	if ierr == nil {
		logger.Println(info)

		r.ApmBuild = info.BuildSha
		r.ApmBuildDate = info.BuildDate
		r.ApmVersion = info.Version
	}

	if initialStatus.SpanIndexCount != nil && finalStatus.SpanIndexCount != nil {
		r.SpansIndexed = *finalStatus.SpanIndexCount - *initialStatus.SpanIndexCount
	}

	if initialStatus.TransactionIndexCount != nil && finalStatus.TransactionIndexCount != nil {
		r.TransactionsIndexed = *finalStatus.TransactionIndexCount - *initialStatus.TransactionIndexCount
	}

	if initialStatus.ErrorIndexCount != nil && finalStatus.ErrorIndexCount != nil {
		r.ErrorsIndexed = *finalStatus.ErrorIndexCount - *initialStatus.ErrorIndexCount
	}

	if initialStatus.Metrics != nil && finalStatus.Metrics != nil {
		memstats := finalStatus.Metrics.Memstats.Sub(initialStatus.Metrics.Memstats)
		logger.Println(memstats)

		r.TotalAlloc = &memstats.TotalAlloc
		r.HeapAlloc = &memstats.HeapAlloc
		r.Mallocs = &memstats.Mallocs
		r.NumGC = &memstats.NumGC

		r.ApmSettings = initialStatus.Metrics.Cmdline.Parse()
	}

	return reports.WithDerivedAttributes(r)
}
