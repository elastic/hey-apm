package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/models"
	"github.com/elastic/hey-apm/worker"
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
func Run(ctx context.Context, input models.Input) error {
	conn, err := es.NewConnection(input.ElasticsearchUrl, input.ElasticsearchAuth)
	if err != nil {
		return errors.Wrap(err, "Elasticsearch not reachable, won't be able to index a report")
	}

	log.Printf("Deleting previous APM event documents...")
	deleted, err := es.DeleteAPMEvents(conn)
	if err != nil {
		return err
	}
	log.Printf("Deleted %d documents", deleted)

	if err := warmUp(ctx, input); err != nil {
		return err
	}

	tests := defineTests(input)
	reports, err := tests.run(ctx)
	if err != nil {
		return err
	}
	if err := verifyReports(reports, conn, input.RegressionMargin, input.RegressionDays); err != nil {
		return err
	}
	return nil
}

func defineTests(input models.Input) tests {
	var t tests
	t.add("transactions only", input.WithTransactions(math.MaxInt32, time.Millisecond*5))
	t.add("small transactions", input.WithTransactions(math.MaxInt32, time.Millisecond*5).WithSpans(10))
	t.add("large transactions", input.WithTransactions(math.MaxInt32, time.Millisecond*5).WithSpans(40))
	t.add("small errors only", input.WithErrors(math.MaxInt32, time.Millisecond).WithFrames(10))
	t.add("very large errors only", input.WithErrors(math.MaxInt32, time.Millisecond).WithFrames(500))
	t.add("transactions only very high load", input.WithTransactions(math.MaxInt32, time.Microsecond*100))
	t.add("transactions, spans and errors high load", input.WithTransactions(math.MaxInt32, time.Millisecond*5).WithSpans(10).WithErrors(math.MaxInt32, time.Millisecond).WithFrames(50))
	return t
}

type test struct {
	name  string
	input models.Input
}

type tests []test

func (t *tests) add(name string, input models.Input) {
	*t = append(*t, test{name: name, input: input})
}

func (t *tests) run(ctx context.Context) ([]models.Report, error) {
	reports := make([]models.Report, len(*t))
	for i, test := range *t {
		log.Printf("running benchmark %q", test.name)
		report, err := worker.Run(ctx, test.input, test.name, nil /*stop*/)
		if err != nil {
			return nil, err
		}
		if err := coolDown(ctx); err != nil {
			return nil, err
		}
		reports[i] = report
	}
	return reports, nil
}

func verifyReports(reports []models.Report, conn es.Connection, margin float64, days string) error {
	var lastErr error
	for _, report := range reports {
		if err := verify(conn, report, margin, days); err != nil {
			fmt.Println(err)
			lastErr = err
		}
	}
	return lastErr
}

// warmUp sends a moderate load to apm-server without saving a report.
func warmUp(ctx context.Context, input models.Input) error {
	input = input.WithErrors(math.MaxInt16, time.Millisecond)
	input.RunTimeout = warm
	input.SkipIndexReport = true
	log.Printf("warming up %.1f seconds...", warm.Seconds())
	if _, err := worker.Run(ctx, input, "warm up", nil); err != nil {
		return err
	}
	return coolDown(ctx)
}

// coolDown waits an arbitrary time for events in elasticsearch be flushed, heap be freed, etc.
func coolDown(ctx context.Context) error {
	log.Printf("cooling down %.1f seconds... ", cool.Seconds())
	timer := time.NewTimer(cool)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// verify asserts there are no performance regressions for a given workload.
//
// compares the given report with saved reports with the same input indexed in the last specified days
// returns an error if connection can't be established,
// or performance decreased by a margin larger than specified
func verify(conn es.Connection, report models.Report, margin float64, days string) error {
	if report.EventsIndexed < 100 {
		return fmt.Errorf("not enough events indexed: %d", report.EventsIndexed)
	}

	filters := []map[string]interface{}{{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": fmt.Sprintf("now-%sd/d", days),
				"lt":  "now",
			},
		},
	}}

	// Convert input to a JSON map, to filter on the previous results for matching inputs.
	inputMap := make(map[string]interface{})
	encodedInput, err := json.Marshal(report.Input)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(encodedInput, &inputMap); err != nil {
		return err
	}
	for k, v := range inputMap {
		filters = append(filters, map[string]interface{}{
			"match": map[string]interface{}{k: v},
		})
	}

	savedReports, fetchErr := es.FetchReports(conn, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
	})
	if fetchErr != nil {
		return fetchErr
	}

	var accPerformance, count float64
	for _, sr := range savedReports {
		if report.ReportId != sr.ReportId {
			accPerformance += sr.Performance()
			count += 1.0
		}
	}
	avgPerformance := accPerformance / count
	if avgPerformance > report.Performance()*margin {
		return newRegression(report, avgPerformance)
	}
	return nil
}

func newRegression(r models.Report, threshold float64) error {
	return errors.New(fmt.Sprintf(
		`test report with doc id %s was expected to show same or better performance than average of %.2f, however %.2f is significantly lower`,
		r.ReportId, threshold, r.Performance(),
	))
}
