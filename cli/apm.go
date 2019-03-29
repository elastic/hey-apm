package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	exec2 "github.com/elastic/hey-apm/exec"
	"github.com/elastic/hey-apm/out"
)

type apm struct {
	version string
	urls    []string
	managed bool
	err     error
	proc    *apmProc
}

type apmProc struct {
	*exec.Cmd
	mu  sync.Mutex
	log []string
}

func newApm(w io.Writer, version string, urls ...string) apm {
	var err error
	if version == "" {
		err = errors.New("apm-server version (version/build sha) required")
	}
	managed := true
	for _, arg := range urls {
		if err == nil {
			if arg == "local" {
				arg = "http://localhost:8200"
			} else {
				_, err = url.ParseRequestURI(arg)
				managed = false
			}
		}
	}

	if managed && err == nil {
		sh := exec2.Shell(w, apmDir(), true)
		sh("git", "checkout", version)
		sh("make")
		_, err = sh("make", "update")
	}

	return apm{version: version, urls: urls, managed: managed, err: err}
}

func apmDir() string {
	return filepath.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/apm-server")
}

// starts apm with the given arguments
func apmStart(w io.Writer, cancel func(), flags []string) (*apmProc, error) {
	apmCmd := apmProc{Cmd: exec.Command("./apm-server", flags...)}
	apmCmd.Dir = apmDir()

	out.ReplyNL(w, out.Grey)
	out.ReplyWithDots(w, apmCmd.Args...)

	// apm-server writes to stderr by default, this consumes it as soon is produced
	stderr, err := apmCmd.StderrPipe()
	if err == nil {
		err = apmCmd.Start()
	}

	scanner := bufio.NewScanner(stderr)
	go func() {
		// assumes default logging configuration
		var log []string
		for scanner.Scan() {
			if len(log) >= 1000 {
				// rotate log
				log = log[:+copy(log[:], append(log[1:], scanner.Text()))]
			} else {
				log = append(log, scanner.Text())
			}
			apmCmd.mu.Lock()
			apmCmd.log = log
			apmCmd.mu.Unlock()
		}
	}()
	go func() {
		time.Sleep(time.Millisecond * 500)
		err := apmCmd.Wait()
		// in case eg. apm server is killed externally (wont work eg. with docker stop)
		cancel()
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			out.Prompt(w)
		}
	}()

	return &apmCmd, err
}

func apmStop(apmCmd *apmProc) error {
	if apmCmd != nil && apmCmd.Process != nil {
		return apmCmd.Process.Kill()
	}
	return errors.New("apm server not running")
}

// returns the last N lines of log up to 1k containing substr, highlighting errors and warnings
func tail(log []string, n int) string {
	w := out.NewBufferWriter()
	n = int(math.Min(float64(n), 1000))
	tailN := int(math.Max(0, float64(len(log)-n)))
	for _, line := range log[tailN:] {
		if strings.Contains(line, "ERROR") || strings.Contains(line, "Error:") {
			out.Reply(w, out.Red)
		} else if strings.Contains(line, "WARN") {
			out.Reply(w, out.Yellow)
		} else {
			out.Reply(w, out.Grey)
		}
		out.ReplyNL(w, line)
	}
	out.ReplyNL(w, out.Yellow, fmt.Sprintf("[time now %s]", time.Now().String()))
	return w.String()
}
