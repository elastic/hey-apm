package worker

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/models"
	"github.com/elastic/hey-apm/server"
)

const quiesceTimeout = 5 * time.Minute

// Run executes a load test work with the given input, prints the results,
// indexes a performance report, and returns it along any error.
//
// If the context is cancelled, the worker exits with the context's error.
// If the stop channel is signalled, the worker exits gracefully with no error.
func Run(ctx context.Context, input models.Input, testName string, stop <-chan struct{}) (models.Report, error) {
	testNode, err := es.NewConnection(input.ApmElasticsearchUrl, input.ApmElasticsearchAuth)
	if err != nil {
		return models.Report{}, errors.Wrap(err, "Elasticsearch used by APM Server not known or reachable")
	}

	worker, err := newWorker(input, stop)
	if err != nil {
		return models.Report{}, err
	}
	logger := worker.logger.Logger
	initialStatus := server.GetStatus(logger, input.ApmServerSecret, input.ApmServerUrl, testNode)

	result, err := worker.work(ctx)
	if err != nil {
		logger.Println(err.Error())
		return models.Report{}, err
	}
	logger.Printf("%s elapsed since event generation completed", result.Flushed.Sub(result.End))
	fmt.Println(result)

	// Wait for apm-server to quiesce before proceeding.
	var finalStatus server.Status
	deadline := time.Now().Add(quiesceTimeout)
	for {
		finalStatus = server.GetStatus(logger, input.ApmServerSecret, input.ApmServerUrl, testNode)
		if finalStatus.Metrics == nil {
			logger.Print("expvar endpoint not available, returning")
			break
		}
		activeEvents := finalStatus.Metrics.LibbeatMetrics.PipelineEventsActive
		if activeEvents == nil || *activeEvents == 0 {
			break
		}
		if !deadline.After(time.Now()) {
			logger.Printf("giving up waiting for %d active events to be processed", *activeEvents)
			break
		}
		logger.Printf("waiting for %d active events to be processed", *activeEvents)
		time.Sleep(time.Second)
	}
	report := createReport(input, testName, result, initialStatus, finalStatus)

	if input.SkipIndexReport {
		return report, err
	}

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

// newWorker returns a new worker with with a workload defined by the input.
func newWorker(input models.Input, stop <-chan struct{}) (*worker, error) {
	logger := newApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))
	tracer, err := agent.NewTracer(logger, input.ApmServerUrl, input.ApmServerSecret, input.APIKey, input.ServiceName, input.SpanMaxLimit)
	if err != nil {
		return nil, err
	}
	return &worker{
		stop:         stop,
		logger:       logger,
		tracer:       tracer,
		RunTimeout:   input.RunTimeout,
		FlushTimeout: input.FlushTimeout,

		TransactionFrequency: input.TransactionFrequency,
		TransactionLimit:     input.TransactionLimit,
		SpanMinLimit:         input.SpanMinLimit,
		SpanMaxLimit:         input.SpanMaxLimit,

		ErrorFrequency:     input.ErrorFrequency,
		ErrorLimit:         input.ErrorLimit,
		ErrorFrameMinLimit: input.ErrorFrameMinLimit,
		ErrorFrameMaxLimit: input.ErrorFrameMaxLimit,
	}, nil
}

func createReport(input models.Input, testName string, result Result, initialStatus, finalStatus server.Status) models.Report {
	this, _ := os.Hostname()
	r := models.Report{
		Input: input,

		ReportId:     shortId(),
		ReportDate:   time.Now().Format(models.GITRFC),
		ReporterHost: this,
		TestName:     testName,

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
		fmt.Println(info)

		r.ApmBuild = info.BuildSha
		r.ApmBuildDate = info.BuildDate
		r.ApmVersion = info.Version
	}

	if initialStatus.Metrics != nil && finalStatus.Metrics != nil {
		memstats := finalStatus.Metrics.Memstats.Sub(initialStatus.Metrics.Memstats)
		fmt.Println(memstats)

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
