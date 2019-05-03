package reports

import (
	"time"

	"github.com/elastic/hey-apm/numbers"
)

type Report struct {
	// Elasticsearch doc id
	ReportId string `json:"report_id"`
	// see GITRFC
	ReportDate string `json:"report_date"`
	// hey-apm host
	ReporterHost string `json:"reporter_host"`
	// like reportDate, but better for querying ES and sorting
	Timestamp time.Time `json:"@timestamp"`
	// agent sending events
	Agent string `json:"agent"`
	// any arbitrary strings set by the user, meant to filter results
	Labels []string `json:"labels"`

	// points to testing cluster, not reporting cluster
	ElasticHost string `json:"elastic_host"`
	// first apm-server host
	ApmHost string `json:"apm_host"`
	// number of apm-servers
	NumApms int `json:"num_apms"`
	// apm-server release version or build sha
	ApmVersion string `json:"apm_version"`
	// commit SHA
	ApmBuild string `json:"apm_build"`
	// commit date
	ApmBuildDate time.Time `json:"apm_build_date"`
	// list of settings apm-server has been started with
	// some are explicitly omitted (eg passwords)
	ApmSettings []string `json:"apm_settings"`

	// specified by the use
	RunTimeout time.Duration `json:"run_timeout"`
	// total elapsed (timeout + flush)
	Elapsed time.Duration `json:"elapsed"`

	// number of total requests to apm-server
	Requests uint64 `json:"requests"`
	// number of total failed requests
	FailedRequests uint64 `json:"failed_requests"`
	// failed / total
	RequestSuccessRatio *float64 `json:"request_success_ratio"`
	// requests per minute
	RequestRate *float64 `json:"request_rate_pm"`

	// total number of responses
	Responses uint64 `json:"responses"`
	// total number of responses
	Responses202 uint64 `json:"responses_202"`
	// total number of responses
	Responses4XX uint64 `json:"responses_4xx"`
	// total number of responses
	Responses5XX uint64 `json:"responses_5xx"`
	// 202 / total
	ResponseSuccessRatio *float64 `json:"response_success_ratio"`

	// number of stacktrace frames per error
	ErrorFrames int `json:"error_frames"`
	// number of errors generated
	ErrorsGenerated uint64 `json:"errors_generated"`
	// error throttling
	ErrorGenerationFrequency time.Duration `json:"error_generation_frequency"`
	// number of errors sent to apm-server
	ErrorsSent uint64 `json:"errors_sent"`
	// number of errors indexed in Elasticsearch
	ErrorsIndexed uint64 `json:"errors_indexed"`
	// sent / generated
	ErrorsSentRatio *float64 `json:"errors_sent_ratio"`
	// 1 - indexed / sent
	ErrorLossRatio *float64 `json:"error_loss_ratio"`

	// number of transactions generated (as per user input)
	TransactionsGenerated uint64 `json:"transactions_generated"`
	// transaction throttling
	TransactionGenerationFrequency time.Duration `json:"transaction_generation_frequency"`
	// number of transactions sent to apm-server
	TransactionsSent uint64 `json:"transactions_sent"`
	// number of transactions indexed in Elasticsearch
	TransactionsIndexed uint64 `json:"transactions_indexed"`
	// sent / generated
	TransactionsSentRatio *float64 `json:"transactions_sent_ratio"`
	// 1 - indexed / sent
	TransactionLossRatio *float64 `json:"transaction_loss_ratio"`

	// number of stacktrace frames per span
	SpanFrames int `json:"span_frames"`
	// spans / transactions
	SpansPerTransaction *float64 `json:"spans_per_transaction"`
	// number of generated spans
	SpansGenerated uint64 `json:"spans_generated"`
	// number of spans sent to apm-server
	SpansSent uint64 `json:"spans_sent"`
	// number of spans indexed in Elasticsearch
	SpansIndexed uint64 `json:"spans_indexed"`
	// sent / generated
	SpansSentRatio *float64 `json:"spans_sent_ratio"`
	// 1 - indexed / sent
	SpanLossRatio *float64 `json:"spans_loss_ratio"`

	// total generated
	EventsGenerated uint64 `json:"events_generated"`
	// total generated per second
	EventGenerationRate *float64 `json:"event_generation_rate"`
	// total sent
	EventsSent uint64 `json:"events_sent"`
	// sent / generated
	EventsSentRatio *float64 `json:"events_sent_ratio"`
	// total sent per second
	EventSendRate *float64 `json:"event_send_rate"`
	// events / requests
	EventsSentPerRequest *float64 `json:"events_per_request"`
	// total accepted
	EventsAccepted uint64 `json:"events_accepted"`
	// accepted / sent
	EventsAcceptedRatio *float64 `json:"events_accepted_ratio"`
	// total accepted per second
	EventAcceptRate *float64 `json:"event_accept_rate"`
	// total indexed
	EventsIndexed uint64 `json:"events_indexed"`
	// indexed / accepted
	EventsIndexedRatio *float64 `json:"events_indexed_ratio"`
	// total indexed per second
	EventIndexRate *float64 `json:"event_index_rate"`
	// 1 - indexed / sent
	EventLossRatio *float64 `json:"event_loss_ratio"`

	// total memory allocated in bytes
	TotalAlloc *float64 `json:"total_alloc"`
	// total memory allocated in the heap, in bytes
	HeapAlloc *float64 `json:"heap_alloc"`
	// total number of mallocs
	Mallocs *int64 `json:"mallocs"`
	// number of GC runs
	NumGC *int64 `json:"num_gc"`
}

func (r Report) date() time.Time {
	t, _ := time.Parse(GITRFC, r.ReportDate)
	return t
}

func WithDerivedAttributes(r Report) Report {
	r.RequestSuccessRatio = numbers.Div(r.Requests, r.Requests+r.FailedRequests)
	r.RequestRate = numbers.Div(r.Requests, r.Elapsed.Minutes())
	r.Responses = numbers.Sum(r.Responses202, r.Responses4XX, r.Responses5XX)
	r.ResponseSuccessRatio = numbers.Div(r.Responses202, r.Responses)
	r.ErrorsSentRatio = numbers.Div(r.ErrorsSent, r.ErrorsGenerated)
	r.ErrorLossRatio = numbers.CPerct(r.ErrorsIndexed, r.ErrorsGenerated)
	r.TransactionsSentRatio = numbers.Div(r.TransactionsSent, r.TransactionsGenerated)
	r.TransactionLossRatio = numbers.CPerct(r.TransactionsIndexed, r.TransactionsGenerated)
	r.SpansPerTransaction = numbers.Div(r.SpansGenerated, r.TransactionsGenerated)
	r.SpansSentRatio = numbers.Div(r.SpansSent, r.SpansGenerated)
	r.SpanLossRatio = numbers.CPerct(r.SpansIndexed, r.SpansGenerated)
	r.EventsGenerated = r.SpansGenerated + r.TransactionsGenerated + r.ErrorsGenerated
	r.EventGenerationRate = numbers.Div(r.ErrorsGenerated, r.Elapsed.Seconds())
	r.EventsSent = numbers.Sum(r.SpansSent, r.TransactionsSent, r.ErrorsSent)
	r.EventSendRate = numbers.Div(r.EventsSent, r.Elapsed.Seconds())
	r.EventsSentRatio = numbers.Div(r.EventsSent, r.EventsGenerated)
	r.EventsSentPerRequest = numbers.Div(r.EventsSent, r.Requests)
	r.EventAcceptRate = numbers.Div(r.EventsAccepted, r.Elapsed.Seconds())
	r.EventsAcceptedRatio = numbers.Div(r.EventsAccepted, r.EventsSent)
	r.EventIndexRate = numbers.Div(r.EventsIndexed, r.Elapsed.Seconds())
	r.EventsIndexedRatio = numbers.Div(r.EventsIndexed, r.EventsAccepted)
	r.EventLossRatio = numbers.CPerct(r.EventsIndexed, r.EventsGenerated)
	return r
}
