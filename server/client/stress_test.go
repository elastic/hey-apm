package client

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/tests"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/elastic/hey-apm/target"
)

var once sync.Once
var environment *evalEnvironment

var noFlags []string

func setupEnv(flags []string) (*evalEnvironment, []string, error) {
	once.Do(func() {
		url := os.Getenv("ELASTICSEARCH_URL")
		usr := os.Getenv("ELASTICSEARCH_USR")
		pwd := os.Getenv("ELASTICSEARCH_PWD")
		environment = NewEvalEnvironment("")
		_, environment.es = elasticSearchUse("", url, usr, pwd)
		apmDir := filepath.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/apm-server")
		_, environment.apm = apmSwitch(console, apmDir, "master", "", []string{"c", "m", "u", "v"})
	})

	flags = apmFlags(*environment.es, environment.apm.Url(), append(flags, "-E", "apm-server.shutdown_timeout=1s"))
	err := apmStop(environment.apm)
	if err == nil {
		time.Sleep(time.Second * 5)
		err, environment.apm = apmStart(console, *environment.apm, func() {}, flags, "-1")
	}
	if err == nil {
		err = waitForServer(environment.apm.Url())
	}
	return environment, flags, err
}

func waitForServer(url string) error {
	c := make(chan error, 1)
	defer close(c)
	go func() {
		for {
			res, err := http.Get(url)
			if err == nil && res.StatusCode == http.StatusOK {
				c <- nil
				return
			}
			time.Sleep(time.Second / 2)
		}
	}()

	select {
	case <-c:
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("timed out waiting for apm-server to start")
	}
}

type consoleWriter struct{}

func (_ *consoleWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write([]byte(tests.WithoutColors(string(p))))
}

var console = &consoleWriter{}

// Executes long running apm-server stress tests, intended for CI
// Tests different workloads with the default apm-server configuration against the master branch
// Expects a fresh apm-server clone in GOPATH and a running Elasticsearch instance, for which connection parameters must be provided
// See `setupEnv` for details
func TestMain(m *testing.M) {
	if os.Getenv("SKIP_STRESS") != "" {
		fmt.Println("skipping apm-server stress tests")
		os.Exit(0)
	}

	env, _, timeoutErr := setupEnv(noFlags)
	defer apmStop(env.apm)

	// bootstrap checks
	if env.es.useErr != nil {
		fmt.Println("elasticsearch instance not available or configured, (missing ELASTICSEARCH_URL / ELASTICSEARCH_USR / ELASTICSEARCH_PWD?)", env.es.useErr)
		os.Exit(1)
	}

	if env.apm.useErr != nil {
		fmt.Println("apm-server not checked out", env.apm.useErr)
		os.Exit(1)
	}

	if running := env.IsRunning(); timeoutErr != nil || running == nil || !*running {
		fmt.Println("could not start apm-server, `make` might have failed or not executed")
		fmt.Println(timeoutErr)
		fmt.Println(api.Tail(env.apm.Log(), 10, ""))
		os.Exit(1)
	}

	if err := apmStop(env.apm); err != nil {
		fmt.Println("could not stop apm-server", err)
		fmt.Println(api.Tail(env.apm.Log(), 10, ""))
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}

// runs a load test with the given workload settings and index the results in elasticsearch as a report
// returns all saved results (reports), including the just indexed; and an error, if occurred
// the error might be related to failing pre-conditions (eg. no apm-server running) or post-conditions
// (eg. no data captured, failed to save the report...)
func doBenchmark(memLimit int64, flags []string, workload ...string) ([]api.TestReport, error) {
	env, flags, err := setupEnv(flags)
	defer reset(env.es)
	if err != nil {
		return nil, err
	}
	block := func() { select {} }
	target, err := target.NewTargetFromOptions("",
		target.NumErrors(workload[0]),
		target.NumTransactions(workload[1]),
		target.NumSpans(workload[2]),
		target.NumFrames(workload[3]),
		target.NumAgents(workload[4]),
		target.Throttle("1000"),
	)
	if err != nil {
		return nil, err
	}
	result := api.LoadTest(console, env, block, *target)
	report := api.NewReport(
		result,
		"hey-apm-tester",
		env.apm.revision,
		env.apm.revDate,
		env.apm.unstaged,
		env.apm.isRemote,
		int64(len(target.Body)),
		maxRssUsed(env.apm.cmd),
		memLimit,
		removeSensitiveFlags(flags),
		console,
	)
	err = report.Error
	if err == nil {
		err = indexReport(env.es.Client, env.es.reportIndex, report)
	}
	if err == nil {
		return env.es.FetchReports()
	}
	return nil, err
}

func assertNoError(t *testing.T, err error) bool {
	if err == nil {
		return true
	}
	return assert.Fail(t, err.Error())
}

func doTest(t *testing.T, flags []string, numErrors, numTransactions, numSpans, numFrames, numAgents string) {
	t.Log("executing apm-server stress test, this will take long. Use SKIP_STRESS=1 to skip it. " +
		"Use -timeout if you want to execute it and need to override the default 10 minutes timeout.")
	duration := "3m"
	memLimit := int64(-1)
	reports, err := doBenchmark(memLimit, flags, duration, numErrors, numTransactions, numSpans, numFrames, numAgents)

	filter := func(k, v string) string {
		return fmt.Sprintf("%s=%s", k, v)
	}

	if assertNoError(t, err) {
		// test no performance regressions since the last week for the same workload
		// variance margin is 1.2 (see `api.MARGIN`), meaning that performance can be 20% worse than other run
		// and the test will still pass
		ok, msg := api.Verify(
			"768h",
			[]string{
				"branch=master",
				filter("duration", duration),
				filter("errors", numErrors),
				filter("transactions", numTransactions),
				filter("spans", numSpans),
				filter("frames", numFrames),
				filter("agents", numAgents),
				fmt.Sprintf("limit=%d", memLimit)},
			reports)
		assert.True(t, ok, msg)
	}
}

func TestSmallTransactionsSequential(t *testing.T) {
	doTest(t, noFlags, "0", "10", "10", "10", "1")
}

func TestSmallErrorsSequential(t *testing.T) {
	doTest(t, noFlags, "10", "0", "0", "10", "1")
}

func TestSmallTransactionsLowNumConcurrentAgents(t *testing.T) {
	doTest(t, noFlags, "0", "10", "10", "10", "5")
}

func TestMediumTransactionsSequential(t *testing.T) {
	doTest(t, noFlags, "0", "20", "20", "20", "1")
}

func TestMediumTransactionsLowNumConcurrentAgents(t *testing.T) {
	doTest(t, noFlags, "0", "20", "20", "20", "5")
}

func TestLargeTransactionsSequential(t *testing.T) {
	doTest(t, noFlags, "0", "30", "30", "30", "1")
}

func TestLargeTransactionsLowNumConcurrentAgents(t *testing.T) {
	doTest(t, noFlags, "0", "30", "30", "30", "5")
}

func TestLargeTransactionsLowNumConcurrentAgentsCustomFlags(t *testing.T) {
	flags := []string{"-E", "output.elasticsearch.bulk_max_size=5000", "-E", "queue.mem.events=5000", "-E", "apm-server.concurrent_requests=10"}
	doTest(t, flags, "0", "30", "30", "30", "5")
}

func TestErrorsVeryHighNumConcurrentAgents(t *testing.T) {
	doTest(t, noFlags, "0", "10", "0", "100", "100")
}
