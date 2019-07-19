package worker

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/models"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/server"
)

// Run executes a load test work with the given input, prints the results,
// indexes a performance report, and returns it along any error.
func Run(input models.Input) (models.Report, error) {
	testNode, err := es.NewConnection(input.ApmElasticsearchUrl, input.ApmElasticsearchAuth)
	if err != nil {
		return models.Report{}, errors.Wrap(err, "Elasticsearch used by APM Server not known or reachable")
	}

	worker := prepareWork(input)
	logger := worker.Logger
	initialStatus := server.GetStatus(logger, input.ApmServerSecret, input.ApmServerUrl, testNode)

	result, err := worker.work()
	if err != nil {
		logger.Println(err.Error())
		return models.Report{}, err
	}
	logger.Printf("%s elapsed since event generation completed", result.Flushed.Sub(result.End))
	fmt.Println(result)

	finalStatus := server.GetStatus(logger, input.ApmServerSecret, input.ApmServerUrl, testNode)
	report := createReport(logger, input, result, initialStatus, finalStatus)

	if input.ElasticsearchUrl == "" {
		logger.Println("es-url unset: not indexing report")
	} else {
		reportNode, _ := es.NewConnection(input.ElasticsearchUrl, input.ElasticsearchAuth)
		if err = es.IndexReport(reportNode, report); err != nil {
			logger.Println(err.Error())
		} else {
			logger.Println("report indexed with document Id " + report.ReportId)
		}
	}
	return report, err
}

// prepareWork returns a worker with with a workload defined by the input.
func prepareWork(input models.Input) worker {

	logger := newApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	tracer := agent.NewTracer(logger, input.ApmServerUrl, input.ApmServerSecret, input.ServiceName, input.SpanMaxLimit)

	w := worker{
		apmLogger:    logger,
		Tracer:       tracer,
		RunTimeout:   input.RunTimeout,
		FlushTimeout: input.FlushTimeout,
	}
	w.addErrors(input.ErrorFrequency, input.ErrorLimit, input.ErrorFrameMinLimit, input.ErrorFrameMaxLimit)
	w.addTransactions(input.TransactionFrequency, input.TransactionLimit, input.SpanMinLimit, input.SpanMaxLimit)
	w.addSignalHandling()

	return w
}

func createReport(logger *log.Logger, input models.Input, result Result, initialStatus, finalStatus server.Status) models.Report {
	this, _ := os.Hostname()
	r := models.Report{
		Input: input,

		ReportId:     shortId(),
		ReportDate:   time.Now().Format(models.GITRFC),
		ReporterHost: this,

		Timestamp: time.Now(),
		Elapsed:   result.Flushed.Sub(result.Start).Seconds(),

		Requests:       result.NumRequests,
		FailedRequests: result.Errors.SendStream,

		ErrorsGenerated: result.ErrorsSent + result.ErrorsDropped,
		ErrorsSent:      result.ErrorsSent,
		ErrorsIndexed:   finalStatus.ErrorIndexCount - initialStatus.ErrorIndexCount,

		TransactionsGenerated: result.TransactionsSent + result.TransactionsDropped,
		TransactionsSent:      result.TransactionsSent,
		TransactionsIndexed:   finalStatus.TransactionIndexCount - initialStatus.TransactionIndexCount,

		SpansGenerated: result.SpansSent + result.SpansDropped,
		SpansSent:      result.SpansSent,
		SpansIndexed:   finalStatus.SpanIndexCount - initialStatus.SpanIndexCount,

		EventsAccepted: result.Accepted,
	}

	info, ierr := server.QueryInfo(input.ApmServerSecret, input.ApmServerUrl)
	if ierr == nil {
		logger.Println(info)

		r.ApmBuild = info.BuildSha
		r.ApmBuildDate = info.BuildDate
		r.ApmVersion = info.Version
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

	return r.WithDerivedAttributes()
}

// shortId returns a short docId for elasticsearch documents. It is not an UUID
func shortId() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b[0:4])
}
