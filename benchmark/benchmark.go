package benchmark

import (
	"fmt"
	"math"
	"time"

	"github.com/elastic/hey-apm/worker"
	"go.elastic.co/apm"

	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/models"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/types"
	"github.com/pkg/errors"
)

const (
	cool = time.Second * 60
	warm = time.Second * 60
)

// Run executes a benchmark test with different workloads against a running apm-server,
// and it checks that are no regressions by comparing it with previous benchmark results
// executed with the same workload.
//
// Regression checks accept an error margin and are not aware of apm-server versions, only URLs.
// apm-server must be started independently with -E apm-server.expvar.enabled=true
func Run(input models.Input, margin float64, days string) error {
	// prevent send on closed chan error
	apm.DefaultTracer.Close()

	conn, err := es.NewConnection(input.ElasticsearchUrl, input.ElasticsearchAuth)
	if err != nil {
		return errors.Wrap(err, "Elasticsearch not reachable, won't be able to index a report")
	}

	warmUp(input)

	run := runner(conn, margin, days)
	run("transactions only", models.Wrap{input}.
		WithTransactions(math.MaxInt32, time.Millisecond*100).
		Input)
	run("small transactions", models.Wrap{input}.
		WithTransactions(math.MaxInt32, time.Millisecond*100).
		WithSpans(10).
		Input)
	run("large transactions", models.Wrap{input}.
		WithTransactions(math.MaxInt32, time.Millisecond*100).
		WithSpans(40).
		Input)
	run("small errors only", models.Wrap{input}.
		WithErrors(math.MaxInt32, time.Millisecond*100).
		WithFrames(10).
		Input)
	run("very large errors only", models.Wrap{input}.
		WithErrors(math.MaxInt32, time.Millisecond*100).
		WithFrames(100).
		Input)
	err = run("high load", models.Wrap{input}.
		WithTransactions(math.MaxInt32, time.Millisecond).
		WithSpans(10).
		Input)

	return err
}

// Runner keeps track of errors during successive calls, returning the last one.
func runner(conn es.Connection, margin float64, days string) func(name string, input models.Input) error {
	var err error
	return func(name string, input models.Input) error {
		fmt.Println("running benchmark with " + name)
		report, e := worker.Run(input)
		if e == nil {
			e = verify(conn, report, margin, days)
		}
		if e != nil {
			fmt.Println(e)
			err = e
		}
		return err
	}
}

// warmUp sends a moderate load to apm-server without saving a report.
func warmUp(input models.Input) {
	input = models.Wrap{input}.WithErrors(math.MaxInt16, time.Millisecond).Input
	input.RunTimeout = warm
	input.ElasticsearchUrl = ""
	fmt.Println(fmt.Sprintf("warming up %.1f seconds...", warm.Seconds()))
	worker.Run(input)
	coolDown()
}

// coolDown waits an arbitrary time for events in elasticsearch be flushed, heap be freed, etc.
func coolDown() {
	fmt.Println(fmt.Sprintf("cooling down %.1f seconds... ", cool.Seconds()))
	time.Sleep(cool)
}

// verify asserts there are no performance regressions for a given workload.
//
// compares the given report with saved reports with the same input indexed in the last specified days
// returns an error if connection can't be established,
// or performance decreased by a margin larger than specified
func verify(conn es.Connection, report models.Report, margin float64, days string) error {
	coolDown()
	if report.EventsIndexed < 100 {
		return errors.New(fmt.Sprintf("not enough events indexed: %d", report.EventsIndexed))
	}

	inputMap := conv.ToMap(report.Input)
	filters := []types.M{
		{
			"range": types.M{
				"@timestamp": types.M{
					"gte": fmt.Sprintf("now-%sd/d", days),
					"lt":  "now",
				},
			},
		},
	}
	for k, v := range inputMap {
		filters = append(filters, types.M{"match": types.M{k: v}})
	}
	body := types.M{
		"query": types.M{
			"bool": types.M{
				"must": filters,
			},
		},
	}

	savedReports, fetchErr := es.FetchReports(conn, body)
	if fetchErr != nil {
		return fetchErr
	}

	var regression error
	for _, sr := range savedReports {
		if report.ReportId != sr.ReportId && report.Performance()*margin < sr.Performance() {
			regression = newRegression(report, sr)
		}
	}
	return regression
}

func newRegression(r1, r2 models.Report) error {
	return errors.New(fmt.Sprintf(`test report with doc id %s was expected to show same or better 
performance as %s, however %.2f is lower than %.2f`,
		r1.ReportId, r2.ReportId, r1.Performance(), r2.Performance()))
}
