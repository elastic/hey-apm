package commands

import (
	"fmt"
	"io"
	"io/ioutil"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/conv"

	"github.com/elastic/hey-apm/compose"
	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/strcoll"
	"github.com/elastic/hey-apm/target"
)

// creates a test workload for the apm-server and returns a test result
// apm-server must be running
// target holds all the configuration for making requests: URL, request body, timeouts, etc.
// blocks current goroutine for as long as `duration` or until wait returns
func LoadTest(w io.Writer, wait func(), cooldown time.Duration, t target.Target) TestResult {
	result := TestResult{Cancelled: true}

	work := t.GetWork(ioutil.Discard)

	start := time.Now()
	go work.Run()
	out.ReplyNL(w, out.Grey+fmt.Sprintf("started new work, payload size %s (uncompressed), %s (compressed) ...",
		conv.ByteCountDecimal(t.Size()), conv.ByteCountDecimal(t.Size())))
	out.Prompt(w)

	cancelled := make(chan struct{}, 1)
	go func() {
		wait()
		cancelled <- struct{}{}
	}()

	select {
	case <-time.After(t.Config.RunTimeout):
		work.Stop()
		time.Sleep(cooldown)
		elapsedTime := time.Now().Sub(start)
		codes := work.StatusCodes()
		_, totalResponses := out.SortedTotal(codes)
		result = TestResult{
			Elapsed:        elapsedTime,
			Flushes:        work.Flushes(),
			TotalResponses: totalResponses,
		}
		out.PrintResults(work, elapsedTime.Seconds(), w)

	case <-cancelled:
		work.Stop()
	}
	return result
}

// writes to disk
func Dump(w io.Writer, args ...string) (int, error) {
	errors, err := conv.Aton(strcoll.Get(0, args), nil)
	transactions, err := conv.Aton(strcoll.Get(1, args), err)
	spans, err := conv.Aton(strcoll.Get(2, args), err)
	frames, err := conv.Aton(strcoll.Get(3, args), err)
	if err != nil {
		return 0, err
	}
	var reqBody = compose.Compose(errors, transactions, spans, frames)
	return w.Write(reqBody)
}

// filters and sorts `reports` and for each result and returns a digest matrix
// each row is the digest of a report with all user-entered attributes equal but one
// for more details check out the Readme and the `reports.collate` function
// TODO add validation and return error
func Collate(size int, since time.Duration, sort string, args []string, reports []TestReport) string {
	variable := strcoll.Get(0, args)
	bw := out.NewBufferWriter()
	digests, err := collate(since, size, variable, sort, strcoll.From(1, args), reports)
	if err != nil {
		out.ReplyEitherNL(bw, err)
	} else {
		for _, group := range digests {
			for _, line := range group {
				out.ReplyNL(bw, s.Join(line, "\t"))
			}
			out.Reply(bw, "\n")
		}
		if len(digests) == 0 {
			out.Reply(bw, "\n")
		}
	}
	return bw.String()
}

// verifies that performance doesn't get worse over time
func Verify(since time.Duration, filterExpr []string, reports []TestReport) (bool, string) {
	bw := out.NewBufferWriter()
	ok, err := verify(since, filterExpr, reports)
	out.ReplyEitherNL(bw, err)
	return ok, bw.String()
}
