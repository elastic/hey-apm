package commands

import (
	"errors"
	"math/rand"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/elastic/hey-apm/exec"
	"github.com/elastic/hey-apm/out"
)

type TestResult struct {
	Cancelled bool `json:"cancelled"`
	// total number of times that the request body has been flushed
	Flushes int64 `json:"request_flushes"`
	// actual elapsed time, used for X-per-second kind of metrics
	Elapsed time.Duration `json:"elapsed"`
	// total number of responses
	TotalResponses int `json:"total_responses"`
}

type TestReport struct {
	// generated programmatically, useful to eg search all fields in elasticsearch
	ReportId string `json:"report_id"`
	// APM Server intake API version
	APIVersion string `json:"api_version"`
	// io.GITRFC
	ReportDate string `json:"report_date"`
	// host in which hey was running when generated this report, empty if `os.Hostname()` failed
	ReporterHost string `json:"reporter_host"`
	// git revision of hey-apm when generated this report, empty if `git rev-parse HEAD` failed
	ReporterRevision string `json:"reporter_revision"`
	// like reportDate, but better for querying ES and sorting
	Timestamp time.Time `json:"@timestamp"`
	// hardcoded to python for now
	Lang string `json:"language"`
	// any error (eg missing data) for which this report shouldn't be saved and considered for data analysis
	Error error
	// any arbitrary strings set by the user, meant to filter results
	Labels string `json:"labels"`

	// points to testing cluster, not reporting cluster
	ElasticHost string `json:"elastic_host"`
	// first apm-server host
	ApmHost string `json:"apm_host"`
	// number of apm-servers tested
	NumApm int `json:"num_apm"`
	// apm-server release version or build sha
	ApmVersion string `json:"apm_version"`
	// TODO add build version
	// specified by the user, used for querying
	Duration time.Duration `json:"duration"`
	// errors per request body
	Errors int `json:"errors"`
	// transactions per request body
	Transactions int `json:"transactions"`
	// spans per transaction
	Spans int `json:"spans"`
	// frames per error and/or span
	Frames int `json:"frames"`
	// Number of concurrent agents sending requests.
	Agents int `json:"agents"`
	// queries per second cap, fixed to a very high number
	Throttle int `json:"throttle"`
	// size in bytes of a single request body, compressed
	GzipBodySize int64 `json:"gzip_body_size"`
	// size in bytes of a single request body, uncompressed
	BodySize int64 `json:"body_size"`
	// request timeout in the agent
	ReqTimeout time.Duration `json:"request_timeout"`
	// whether it streams events or not
	Stream bool `json:"stream"`
	// number of elasticsearch docs encoded in the JSON body of the request
	DocsPerRequest int64 `json:"docs_per_request"`

	// total number of elasticsearch docs indexed
	ActualDocs int64 `json:"actual_indexed_docs"`
	// total pushed volume, uncompressed, in bytes
	Pushed int64 `json:"pushed"`
	// total pushed volume, compressed, in bytes
	GzipPushed int64 `json:"gzip_pushed"`
	// number of requests per second
	PushedRps float64 `json:"pushed_rps"`
	// pushed volume per second, uncompressed, in bytes
	PushedBps float64 `json:"pushed_bps"`
	// number of docs indexed per second
	Throughput float64 `json:"throughput"`

	TestResult
}

func NewReport(result TestResult, labels, esUrl, apmUrl, apmVersion string, numApm, indexCount int) TestReport {
	r := TestReport{
		Lang:        "python",
		APIVersion:  "v2",
		ReportId:    randId(time.Now().Unix()),
		ReportDate:  time.Now().Format(GITRFC),
		Timestamp:   time.Now(),
		Throughput:  float64(indexCount) / result.Elapsed.Seconds(),
		Labels:      labels,
		ElasticHost: host(esUrl),
		ApmHost:     host(apmUrl),
		NumApm:      numApm,
		ApmVersion:  apmVersion,
		ActualDocs:  int64(indexCount),
		TestResult:  result,
	}

	if r.Cancelled {
		r.Error = errors.New("test run cancelled")
	} else if r.Duration.Seconds() < 10 {
		r.Error = errors.New("test duration too short")
	}

	r.ReporterHost, _ = os.Hostname()
	selfDir := path.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/hey-apm")
	if rRev, err := exec.Shell(out.NewBufferWriter(), selfDir, false)("git", "rev-parse", "HEAD"); err != nil {
		r.ReporterRevision = rRev
	}

	return r
}

func host(urlStr string) string {
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	return url.Hostname()
}

func (r TestReport) labels() []string {
	if len(r.Labels) > 0 {
		return strings.Split(r.Labels, ",")
	}
	return nil
}

func (r TestReport) date() time.Time {
	t, _ := time.Parse(GITRFC, r.ReportDate)
	return t
}

func randId(seed int64) string {
	rand.Seed(seed)
	l := 8
	runes := []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, l)
	for i := 0; i < l; i++ {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}
