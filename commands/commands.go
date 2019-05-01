package commands

import (
	"io"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/worker"

	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/strcoll"
)

// TODO
func LoadTest(w io.Writer, wait func(), cooldown time.Duration, worker worker.Worker) TestResult {
	result := TestResult{Cancelled: true}
	worker.Run()
	return result
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
