package models

import (
	"time"
)

// Input holds all the parameters given to a load test work.
// Most parameters describe a workload pattern, others are required to create performance reports.
type Input struct {

	// Whether or not this object will be processed by the `benchmark` package
	IsBenchmark bool `json:"-"`
	// Number of days to look back for regressions (only if IsBenchmark is true)
	RegressionDays string `json:"-"`
	// Acceptable performance decrease without being considered as regressions, as a percentage
	// (only if IsBenchmark is true)
	RegressionMargin float64 `json:"-"`

	// URL of the APM Server under test
	ApmServerUrl string `json:"apm_url"`
	// Secret token of the APM Server under test
	ApmServerSecret string `json:"-"`
	// API Key for communication between APM Server and the Go agent
	APIKey string `json:"-"`
	// If true, it will index the performance report of a run in ElasticSearch
	SkipIndexReport bool `json:"-"`
	// URL of the Elasticsearch instance used for indexing the performance report
	ElasticsearchUrl string `json:"-"`
	// <username:password> of the Elasticsearch instance used for indexing the performance report
	ElasticsearchAuth string `json:"-"`
	// URL of the Elasticsearch instance used by APM Server
	ApmElasticsearchUrl string `json:"elastic_url,omitempty"`
	// <username:password> of the Elasticsearch instance used by APM Server
	ApmElasticsearchAuth string `json:"-"`
	// Service name passed to the tracer
	ServiceName string `json:"service_name,omitempty"`

	// Run timeout of the performance test (ends the test when reached)
	RunTimeout time.Duration `json:"run_timeout"`
	// Timeout for flushing the workload to APM Server
	FlushTimeout time.Duration `json:"flush_timeout"`
	// Frequency at which the tracer will generate transactions
	TransactionFrequency time.Duration `json:"transaction_generation_frequency"`
	// Maximum number of transactions to push to the APM Server (ends the test when reached)
	TransactionLimit int `json:"transaction_generation_limit"`
	// Maximum number of spans per transaction
	SpanMaxLimit int `json:"spans_generated_max_limit"`
	// Minimum number of spans per transaction
	SpanMinLimit int `json:"spans_generated_min_limit"`
	// Frequency at which the tracer will generate errors
	ErrorFrequency time.Duration `json:"error_generation_frequency"`
	// Maximum number of errors to push to the APM Server (ends the test when reached)
	ErrorLimit int `json:"error_generation_limit"`
	// Maximum number of stacktrace frames per error
	ErrorFrameMaxLimit int `json:"error_generation_frames_max_limit"`
	// Minimum number of stacktrace frames per error
	ErrorFrameMinLimit int `json:"error_generation_frames_min_limit"`
}

type Wrap struct {
	Input
}

func (w Wrap) WithErrors(limit int, freq time.Duration) Wrap {
	w.ErrorLimit = limit
	w.ErrorFrequency = freq
	return w
}

func (w Wrap) WithFrames(f int) Wrap {
	w.ErrorFrameMaxLimit = f
	w.ErrorFrameMinLimit = f
	return w
}

func (w Wrap) WithTransactions(limit int, freq time.Duration) Wrap {
	w.TransactionLimit = limit
	w.TransactionFrequency = freq
	return w
}

func (w Wrap) WithSpans(s int) Wrap {
	w.SpanMaxLimit = s
	w.SpanMinLimit = s
	return w
}
