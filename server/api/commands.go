package api

import (
	"errors"
	"fmt"
	stdio "io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/compose"
	"github.com/elastic/hey-apm/output"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/strcoll"
	"github.com/elastic/hey-apm/target"
)

// creates a test workload for the apm-server and returns a string to be printed and a report to be saved
// apm-server must be running
// target holds all the configuration for making requests: URL, request body, timeouts, etc.
// blocks current goroutine for as long as `duration` or until waitForCancel returns
func LoadTest(w stdio.Writer, state State, waitForCancel func(), t target.Target) TestResult {
	result := TestResult{Cancelled: true}

	// apm-server warm up
	time.Sleep(time.Second)
	if err := state.Ready(); err != nil {
		io.ReplyEither(w, err)
		return result
	}

	uncompressed := int64(len(t.Body))
	work := t.GetWork(ioutil.Discard)

	docsBefore := state.ElasticSearch().Count()
	start := time.Now()
	go work.Run()
	io.ReplyNL(w, io.Grey+fmt.Sprintf("started new work, url %s, payload size %s (uncompressed), %s (compressed) ...",
		t.Url, byteCountDecimal(uncompressed), byteCountDecimal(int64(len(t.Body)))))
	io.Prompt(w)

	cancelled := make(chan struct{}, 1)
	go func() {
		waitForCancel()
		cancelled <- struct{}{}
	}()

	select {
	case <-time.After(t.Config.RunTimeout):
		work.Stop()
		elapsedTime := time.Now().Sub(start)
		codes := work.StatusCodes()
		_, totalResponses := output.SortedTotal(codes)
		result = TestResult{
			Elapsed:        elapsedTime,
			Duration:       t.Config.RunTimeout,
			Errors:         t.Config.NumErrors,
			Transactions:   t.Config.NumTransactions,
			Spans:          t.Config.NumSpans,
			Frames:         t.Config.NumFrames,
			DocsPerRequest: int64(t.Config.NumErrors+t.Config.NumTransactions+(t.Config.NumTransactions*t.Config.NumSpans)) * work.Flushes(),
			Agents:         t.Config.NumAgents,
			Throttle:       int(t.Config.Throttle),
			Stream:         t.Config.Stream,
			GzipBodySize:   int64(len(t.Body)),
			BodySize:       uncompressed,
			Pushed:         uncompressed * work.Flushes(),
			GzipPushed:     int64(len(t.Body)) * work.Flushes(),
			Flushes:        work.Flushes(),
			ReqTimeout:     time.Duration(t.Config.RequestTimeout),
			ElasticUrl:     state.ElasticSearch().Url(),
			ApmUrl:         state.ApmServer().Url(),
			ApmHost:        apmHost(state.ApmServer().Url()),
			Branch:         state.ApmServer().Branch(),
			// AcceptedResponses: codes[202],
			TotalResponses: totalResponses,
			ActualDocs:     state.ElasticSearch().Count() - docsBefore,
		}

		var format string
		var throttled string
		if t.Config.Stream {
			if result.Throttle < math.MaxInt16 {
				throttled = fmt.Sprintf("throttled at %d events per second", result.Throttle)
			}
			format = "%s\nstreamed %d events per request with %d agent(s) %s\n%s"
		} else {
			if result.Throttle < math.MaxInt16 {
				throttled = fmt.Sprintf("throttled at %d requests per second", result.Throttle)
			}
			format = "%s\nsent %d events per request with %d agent(s) %s\n%s"
		}
		io.ReplyNL(w, fmt.Sprintf(format, io.Yellow, result.DocsPerRequest, result.Agents, throttled, io.Grey))
		output.PrintResults(work, elapsedTime.Seconds(), w)

	case <-cancelled:
		work.Stop()
	}
	return result
}

// returns the last N lines of log up to 1k containing substr, highlighting errors and warnings
func Tail(log []string, N int, subStr string) string {
	w := io.NewBufferWriter()
	ret := make([]string, 0)
	for _, line := range log {
		if s.Contains(line, subStr) {
			ret = append(ret, line)
		}
	}
	N = int(math.Min(float64(N), 1000))
	tailN := int(math.Max(0, float64(len(ret)-N)))
	for _, line := range ret[tailN:] {
		if s.Contains(line, "ERROR") || s.Contains(line, "Error:") {
			io.Reply(w, io.Red)
		} else if s.Contains(line, "WARN") {
			io.Reply(w, io.Yellow)
		} else {
			io.Reply(w, io.Grey)
		}
		io.ReplyNL(w, line)
	}
	io.ReplyNL(w, io.Yellow, fmt.Sprintf("[time now %s]", time.Now().String()))
	return w.String()
}

// returns formatted name definitions containing `match` in either left or right side, or all if `match` is empty
func NameDefinitions(nameDefs map[string][]string, match string) string {
	w := io.NewBufferWriter()
	if len(nameDefs) == 0 {
		io.ReplyNL(w, io.Grey+"nothing to show")
		return w.String()
	}
	keys := make([]string, 0)
	for k := range nameDefs {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))
	for _, k := range keys {
		v := nameDefs[k]
		cmd := s.Join(v, " ")
		if match == "" || s.Contains(k, match) || s.Contains(cmd, match) {
			io.ReplyNL(w, io.Magenta+k+io.Grey+" "+cmd)
		}
	}
	return w.String()
}

// defines a name and returns a new name definitions map
// a name definition maps 1 word to several words
// cmd examples:
// `ma apm switch master` maps `ma` to `apm switch master`
// `fe -E apm-server.frontend=true` maps `fe` to `-E apm-server.frontend=true`
// `mafe ma ; test 1s 1 1 1 1 fe` (invoking `mafe` will run `apm switch master`, then `test 1s 1 1 1 1 -E apm-server.frontend=true`)
// `rm fe` (will remove the `fe` definition and cause `mafe` invocations to fail)
func Define(usr string, fw io.FileWriter, reserved, cmd []string, nameDefs map[string][]string) (string, map[string][]string) {
	var err error
	out := "ok"
	m := strcoll.Copy(nameDefs)
	w := io.NewBufferWriter()
	left, right := strcoll.Nth(0, cmd), strcoll.Rest(1, cmd)
	if left == "rm" {
		// this might leave dangling names, todo delete them
		delete(m, strcoll.Nth(0, right))
		err = io.StoreDefs(usr, fw, m)
	} else {
		if strcoll.Contains(left, reserved) {
			err = errors.New(left + " is a reserved word")
		} else if strcoll.Contains(left, right) {
			err = errors.New(left + " can't appear in the right side")
		} else {
			if v, ok := m[left]; ok {
				out = "updated old value: " + s.Join(v, " ")
			}
			m[left] = right
			err = io.StoreDefs(usr, fw, m)
		}
	}
	io.ReplyEither(w, err, io.Grey+out)
	return w.String(), m
}

func Status(state State) *io.BufferWriter {
	w := io.NewBufferWriter()
	// es status
	elasticSearch := state.ElasticSearch()
	if elasticSearch.Url() == "" {
		io.ReplyNL(w, io.Grey+fmt.Sprintf(
			"ElasticSearch: %snot configured %s(hint: elasticsearch use <url>)", io.Red, io.Grey))
	} else {
		health, err := elasticSearch.Health()
		io.ReplyEitherNL(w, err, io.Grey+fmt.Sprintf("ElasticSearch [%s]: %s , %s %d docs",
			elasticSearch.Url(), health, io.Grey, elasticSearch.Count()))
	}
	// apm-server process status
	apmServer := state.ApmServer()
	apmStatus := io.Yellow + "not managed by hey-apm"
	if running := apmServer.IsRunning(); running != nil && *running {
		apmStatus = io.Magenta + "running" + io.Grey
	} else if running != nil && !*running {
		apmStatus = io.Green + "not running"
	}
	io.ReplyNL(w, io.Grey+fmt.Sprintf("ApmServer [%s]: %s", apmServer.Url(), apmStatus))

	// apm-server repo status
	// todo it would be better to expose useErr and print that instead
	if err := os.Chdir(apmServer.Dir()); apmServer.Dir() != "" && err != nil {
		io.ReplyNL(w, io.Red+fmt.Sprintf("Can't ch to directory %s", apmServer.Dir())+io.Grey+" (hint: apm use <dir>)")
		return w
	}
	var branch string
	if apmServer.Branch() == "" {
		branch = io.Red + "unknown branch" + io.Grey + " (hint: apm switch <branch>)"
	} else {
		branch = io.Green + apmServer.Branch() + io.Grey + ", " + apmServer.PrettyRevision()
	}
	io.ReplyNL(w, io.Grey+fmt.Sprintf("Using %s: %s", apmServer.Dir(), branch))
	return w
}

// writes to disk
func Dump(fw io.FileWriter, fileName string, args ...string) string {
	w := io.NewBufferWriter()
	errors, err := atoi(strcoll.Nth(0, args), nil)
	transactions, err := atoi(strcoll.Nth(1, args), err)
	spans, err := atoi(strcoll.Nth(2, args), err)
	frames, err := atoi(strcoll.Nth(3, args), err)
	if err != nil {
		io.ReplyEitherNL(w, err)
		return w.String()
	}
	var reqBody = compose.Compose(errors, transactions, spans, frames)
	err = fw.WriteToFile(fileName, reqBody)
	io.ReplyEitherNL(w, err, io.Grey+byteCountDecimal(int64(len(reqBody)))+" written to disk")
	return w.String()
}

func Help() string {
	w := io.NewBufferWriter()
	io.ReplyNL(w, io.Yellow+"commands might be entered separated by semicolons, (eg: \"apm use last ; status\")")
	io.ReplyNL(w, io.Magenta+"status")
	io.ReplyNL(w, io.Grey+"    shows elasticsearch and apm-server current status, and queued commands")
	io.ReplyNL(w, io.Magenta+"elasticsearch use [<url> <username> <password> | last | local]")
	io.ReplyNL(w, io.Grey+"    connects to an elasticsearch node with given parameters")
	io.ReplyNL(w, io.Magenta+"        last"+io.Grey+" uses the last working parameters")
	io.ReplyNL(w, io.Magenta+"        local"+io.Grey+" short for http://localhost:9200")
	io.ReplyNL(w, io.Magenta+"elasticsearch reset")
	io.ReplyNL(w, io.Grey+"    deletes all the apm-* indices")
	io.ReplyNL(w, io.Magenta+"apm use [<dir> | <url> | last | docker | local]")
	io.ReplyNL(w, io.Grey+"    informs the location of the apm-server repo")
	io.ReplyNL(w, io.Magenta+"        last"+io.Grey+" uses the last working directory")
	io.ReplyNL(w, io.Magenta+"        docker"+io.Grey+" builds and runs apm-server inside a docker container")
	io.ReplyNL(w, io.Magenta+"        local"+io.Grey+" short for GOPATH/src/github.com/elastic/apm-server")
	io.ReplyNL(w, io.Magenta+"        <url>"+io.Grey+" this will cause hey-apm to not manage apm-server, test reports won't be saved")
	io.ReplyNL(w, io.Magenta+"        <dir>"+io.Grey+" if the location is not a valid URL, it will be considered a local directory")
	io.ReplyNL(w, io.Magenta+"apm list")
	io.ReplyNL(w, io.Grey+"    shows the docker images created by apm-server")
	io.ReplyNL(w, io.Magenta+"apm switch <branch> [<revision> <OPTIONS>...]")
	io.ReplyNL(w, io.Grey+"    informs hey-apm to use the specified branch and revision, it doesn't have doEffect if apm-server is not managed by hey-apm")
	io.ReplyNL(w, io.Grey+"    OPTIONS:")
	io.ReplyNL(w, io.Magenta+"        -f, --fetch"+io.Grey+" runs git fetch on apm-server")
	io.ReplyNL(w, io.Magenta+"        -c, --checkout"+io.Grey+" runs git checkout <branch> [<revision>] on apm-server")
	io.ReplyNL(w, io.Magenta+"        -u, --make-update"+io.Grey+" runs make update")
	io.ReplyNL(w, io.Magenta+"        -m, --make"+io.Grey+" runs make")
	io.ReplyNL(w, io.Magenta+"        -v, --verbose"+io.Grey+" shows the output")
	io.ReplyNL(w, io.Grey+"    when using docker, the only applicable option is -v, all the others are implicitly used")
	io.ReplyNL(w, io.Magenta+"test <duration> <transactions> <spans> <frames> <agents> [<apmserver-flags> <OPTIONS>...]")
	io.ReplyNL(w, io.Grey+"    starts the apm-server and performs a workload test against it")
	io.ReplyNL(w, io.Magenta+"        <duration>"+io.Grey+" duration of the load test (eg \"1m\")")
	io.ReplyNL(w, io.Magenta+"        <transactions>"+io.Grey+" transactions per request body")
	io.ReplyNL(w, io.Magenta+"        <spans>"+io.Grey+" spans per transaction")
	io.ReplyNL(w, io.Magenta+"        <frames>"+io.Grey+" frames per document (for spans and errors)")
	io.ReplyNL(w, io.Magenta+"        <apmserver-flags>"+io.Grey+" any flags passed to apm-server (elasticsearch url/username/password and apm-server url are overwritten), it doesn't have doEffect if apm-server is not managed by hey-apm")
	io.ReplyNL(w, io.Grey+"    OPTIONS:")
	io.ReplyNL(w, io.Magenta+"        --errors <errors>"+io.Grey+" number of errors per request body")
	io.ReplyNL(w, io.Grey+"        defaults to 0")
	io.ReplyNL(w, io.Magenta+"        --stream"+io.Grey+" use the streaming protocol")
	io.ReplyNL(w, io.Magenta+"        --agents <agents>"+io.Grey+" number of simultaneous agents sending queries")
	io.ReplyNL(w, io.Grey+"        defaults to 1")
	io.ReplyNL(w, io.Magenta+"        --throttle <throttle>"+io.Grey+" upper limit of queries per second to send")
	io.ReplyNL(w, io.Grey+"        defaults to 1")
	io.ReplyNL(w, io.Magenta+"        --timeout <timeout>"+io.Grey+" client request timeout")
	io.ReplyNL(w, io.Grey+"        defaults to 10s")
	io.ReplyNL(w, io.Magenta+"        --mem <mem-limit>"+io.Grey+" memory limit passed to docker run, it doesn't have effect if apm-server is not dockerized")
	io.ReplyNL(w, io.Grey+"        defaults to 4g")
	io.ReplyNL(w, io.Magenta+"apm tail [-<n> <pattern>]")
	io.ReplyNL(w, io.Grey+"    shows the last lines of the apm server log")
	io.ReplyNL(w, io.Magenta+"        -<n>"+io.Grey+" shows the last <n> lines up to 1000, defaults to 10")
	io.ReplyNL(w, io.Magenta+"        <pattern>"+io.Grey+" shows only the lines matching the pattern (no regex support)")
	io.ReplyNL(w, io.Magenta+"cancel [<command_id>]")
	io.ReplyNL(w, io.Grey+"    cancels the ongoing workload test, if any")
	io.ReplyNL(w, io.Magenta+"         <command_id>"+io.Grey+" cancels all the queued commands with the given id")
	io.ReplyNL(w, io.Magenta+"collate <VARIABLE> [-n <n> --csv --sort <CRITERIA> <FILTER>...]")
	io.ReplyNL(w, io.Grey+"    queries reports generated by workload tests, and per each result shows other reports in which only VARIABLE is different")
	io.ReplyNL(w, io.Magenta+"        -n <n>"+io.Grey+" shows up to <n> report groups if n is a number, or since n time ago if n is a duration")
	io.ReplyNL(w, io.Grey+"    defaults to 20")
	io.ReplyNL(w, io.Magenta+"        --sort <CRITERIA>"+io.Grey+" sorts the results by the given CRITERIA, defaults to report_date")
	io.ReplyNL(w, io.Magenta+"        --csv"+io.Grey+" separate fields by tabs, without aligning rows and without truncating results")
	io.ReplyNL(w, io.Grey+"        CRITERIA:")
	io.ReplyNL(w, io.Magenta+"                report_date"+io.Grey+" date of the generated report, most recent first")
	io.ReplyNL(w, io.Magenta+"                revision_date"+io.Grey+" date of the git commit, most recent first")
	io.ReplyNL(w, io.Magenta+"                duration"+io.Grey+" duration of the workload test, higher first")
	io.ReplyNL(w, io.Magenta+"                pushed_volume"+io.Grey+" bytes pushed per second, higher first")
	// io.ReplyNL(w, io.Magenta+"                actual_expected_ratio"+io.Grey+" ratio between actual and expected docs indexed, higher first")
	// io.ReplyNL(w, io.Magenta+"                latency"+io.Grey+" milliseconds per accepted request, lower first")
	io.ReplyNL(w, io.Magenta+"                throughput"+io.Grey+" indexed documents per second, higher first")
	io.ReplyNL(w, io.Magenta+"                efficiency"+io.Grey+" documents indexed per minute per megabyte of used RAM, higher first")
	io.ReplyNL(w, io.Magenta+"        <VARIABLE>"+io.Grey+" shows together reports generated from workload tests with the same parameters except VARIABLE")
	io.ReplyNL(w, io.Grey+"        VARIABLE:")
	io.ReplyNL(w, io.Magenta+"                duration"+io.Grey+" duration of the test")
	io.ReplyNL(w, io.Magenta+"                transactions"+io.Grey+" transactions per request body")
	io.ReplyNL(w, io.Magenta+"                errors"+io.Grey+" errors per request body")
	io.ReplyNL(w, io.Magenta+"                spans"+io.Grey+" spans per transaction")
	io.ReplyNL(w, io.Magenta+"                frames"+io.Grey+" frames per document")
	io.ReplyNL(w, io.Magenta+"        		   stream"+io.Grey+" whether a test used the streaming protocol or not")
	io.ReplyNL(w, io.Magenta+"                agents"+io.Grey+" number of concurrent agents")
	io.ReplyNL(w, io.Magenta+"        		   throttle"+io.Grey+" upper limit of queries per second sent")
	io.ReplyNL(w, io.Magenta+"        		   timeout"+io.Grey+" request timeout")
	io.ReplyNL(w, io.Magenta+"                branch"+io.Grey+" git branch and commit (if the branch is variable, the revision necessarily varies too)")
	io.ReplyNL(w, io.Magenta+"                revision"+io.Grey+" git commit")
	io.ReplyNL(w, io.Magenta+"                limit"+io.Grey+" memory limit passed to docker")
	io.ReplyNL(w, io.Magenta+"                <flag>"+io.Grey+" flag passed to the apm-server with -E")
	io.ReplyNL(w, io.Magenta+"        <FILTER>"+io.Grey+" returns only reports matching all given filters, specified like <FIELD>=|!=|<|><value>")
	io.ReplyNL(w, io.Grey+"        dates must be formatted like \"2018-28-02\" and durations like \"1m\"")
	io.ReplyNL(w, io.Grey+"        strings do not support <,> comparators")
	io.ReplyNL(w, io.Magenta+"        FIELDs"+io.Grey+"  any VARIABLE attribute, or:")
	io.ReplyNL(w, io.Magenta+"                report_id"+io.Grey+" unique id generated for each report")
	io.ReplyNL(w, io.Magenta+"                report_date"+io.Grey+" date of the generated report")
	io.ReplyNL(w, io.Magenta+"                request_size"+io.Grey+" number of bytes in the request body")
	io.ReplyNL(w, io.Magenta+"                revision_date"+io.Grey+" date of the git commit")
	io.ReplyNL(w, io.Grey+"        command example: \"collate -24h revision branch=master revision_date>2018-28-02 agents=10 duration<5m --sort efficiency\"")
	io.ReplyNL(w, io.Magenta+"verify -n <n> <FILTER>...")
	io.ReplyNL(w, io.Grey+"    verifies that there is not a negative trend over time")
	io.ReplyNL(w, io.Grey+"    (apm-server flags might skew results)")
	io.ReplyNL(w, io.Magenta+"        -n <n>"+io.Grey+" verifies the up to last <n> reports if n is a number, or since n time ago if n is a duration")
	io.ReplyNL(w, io.Grey+"    defaults to 168h (1 week)")
	io.ReplyNL(w, io.Magenta+"    FILTERS"+io.Grey+" are specified like <FIELD>=|!=|<|><value>")
	io.ReplyNL(w, io.Grey+"    all FIELDS are required: duration, errors, transactions, spans, frames, stream, agents, throttle, timeout, branch, limit")
	io.ReplyNL(w, io.Magenta+"define [<pattern> | <name> <sequence> | rm <name>]")
	io.ReplyNL(w, io.Grey+"    without arguments, shows the current saved name definitions")
	io.ReplyNL(w, io.Magenta+"       <pattern>"+io.Grey+"  shows the current saved name definitions matching the pattern (no regex support)")
	io.ReplyNL(w, io.Magenta+"       <name> <sequence>"+io.Grey+"   alias a sequence of strings to the given name")
	io.ReplyNL(w, io.Grey+"       sequence can be any string(s) supporting $ placeholders for variable substitution, semicolons should be surrounded by spaces")
	io.ReplyNL(w, io.Magenta+"       rm <name>"+io.Grey+"  removes given name")
	io.ReplyNL(w, io.Magenta+"dump <file_name> <errors> <transactions> <spans> <frames>")
	io.ReplyNL(w, io.Grey+"    writes to <file_name> a payload with the given profile (described above)")
	io.ReplyNL(w, io.Magenta+"help")
	io.ReplyNL(w, io.Grey+"    shows this help")
	io.ReplyNL(w, io.Magenta+"quit")
	io.ReplyNL(w, io.Grey+"    quits this connection")
	io.ReplyNL(w, io.Magenta+"exit")
	io.ReplyNL(w, io.Grey+"    same as quit")
	return w.String()
}

// filters and sorts `reports` and for each result and returns a digest matrix
// each row is the digest of a report with all user-entered attributes equal but one
// for more details check out the Readme and the `reports.collate` function
func Collate(ND, sort string, csv bool, args []string, reports []TestReport) string {
	variable := strcoll.Nth(0, args)
	// we cant have a whitelist of variables because flags are unknown, but we can do some basic check
	if variable == "" ||
		s.Contains(variable, "=") ||
		s.Contains(variable, ">") ||
		s.Contains(variable, "<") {
		return io.Red + "<variable> argument is required\n"
	}
	bw := io.NewBufferWriter()
	digests, err := collate(ND, variable, sort, !csv, strcoll.Rest(1, args), reports)
	if err != nil {
		io.ReplyEitherNL(bw, err)
	} else {
		for _, group := range digests {
			for _, line := range group {
				if csv {
					io.ReplyNL(bw, io.Grey+s.Join(line, "\t"))
				} else {
					io.ReplyNL(bw, io.Grey+s.Join(line, "  "))
				}
			}
			io.ReplyNL(bw, io.Grey)
		}
		if len(digests) == 0 {
			io.Reply(bw, io.Grey+"\n")
		}
	}
	return bw.String()
}

// verifies that performance doesn't get worse over time
func Verify(since string, filterExpr []string, reports []TestReport) (bool, string) {
	bw := io.NewBufferWriter()
	ok, out, err := verify(since, filterExpr, reports)
	io.ReplyEitherNL(bw, err, out)
	return ok, bw.String()
}

func atoi(attr string, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(attr)
	if n < 0 {
		err = errors.New("negative values not allowed")
	}
	return n, err
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
