package exec

import (
	"io"
	"os/exec"
	"strings"

	"github.com/elastic/hey-apm/out"

	"github.com/elastic/hey-apm/util"
)

// returns a function that sends its arguments to exec.Cmd, tracking occurred errors across invocations and
// aborting if a previous error had occurred
func Shell(w io.Writer, dir string, verbose bool) func(...string) (string, error) {
	var err error
	return func(args ...string) (string, error) {
		if err != nil || util.Get(0, args) == "" {
			return "", err
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if verbose {
			out.ReplyWithDots(w, cmd.Args...)
		}
		var bytes []byte
		// todo use io.Pipe() to get real time stdout/stderr
		// useful when w is the tcp connection and commands take a while
		bytes, err = cmd.CombinedOutput()
		ret := strings.TrimSpace(string(bytes))
		if err != nil {
			// errors are always printed
			out.ReplyNL(w, out.Red+ret)
		} else if verbose {
			out.ReplyNL(w, out.Grey+ret)
		}
		return ret, err
	}
}
