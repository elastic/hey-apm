package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/elastic/hey-apm/cli"
	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/requester"
	"github.com/elastic/hey-apm/target"
)

var (
	// run options
	runTimeout  = flag.Duration("run", 30*time.Second, "stop run after this duration")
	maxRequests = flag.Int("requests", math.MaxInt32, "maximum requests to make")

	// payload options
	numAgents       = flag.Int("c", 1, "number of agents sending data concurrently")
	qps             = flag.Float64("q", 0, "queries per second")
	pause           = flag.Duration("p", 1*time.Millisecond, "Only used if `qps` is not set. Defines the pause between sending events over the same http request.")
	numErrors       = flag.Int("e", 3, "number of errors")
	numTransactions = flag.Int("t", 6, "number of transactions")
	numFrames       = flag.Int("f", 20, "number of stacktrace frames per span")
	numSpans        = flag.Int("s", 7, "number of spans")

	// http options
	baseUrl            = flag.String("base-url", "http://localhost:8200", "")
	endpoint           = flag.String("endpoint", "/intake/v2/events", "")
	method             = flag.String("method", "POST", "method type")
	headers            = newStringsOpt("header", []string{}, "header(s) added to all requests")
	requestTimeout     = flag.Duration("request-timeout", 10*time.Second, "request timeout in seconds")
	idleTimeout        = flag.Duration("idle-timeout", 3*time.Minute, "idle timeout")
	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")

	interactive = flag.Bool("cli", false, "run hey-apm in interactive mode listening on 8234")
	stream      = flag.Bool("stream", false, "send data in a streaming way via http")
)

type stringsOpt struct {
	val []string
	set bool
}

func (s *stringsOpt) String() string {
	return fmt.Sprint(*s)
}

func (s *stringsOpt) Set(value string) error {
	if !s.set {
		(*s).val = []string{}
	}
	s.set = true
	(*s).val = append((*s).val, value)
	return nil
}

func (s *stringsOpt) List() []string {
	return []string((*s).val)
}

func newStringsOpt(name string, value []string, usage string) *stringsOpt {
	var s stringsOpt
	for _, v := range value {
		s.val = append(s.val, v)
	}
	flag.Var(&s, name, usage)
	return &s
}

func main() {
	flag.Parse()

	logger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

	if *interactive {
		logger.Println("Starting hey-apm in interactive mode...")
		logger.Println("Connect with 'rlwrap telnet localhost 8234'")
		logger.Println("WARNING: Multiple concurrent tests against the same apm-server and/or elasticsearch instances will interfere with each other")
		cli.Serve()
		os.Exit(0)
	}

	header := make(http.Header)
	for _, h := range headers.List() {
		kv := strings.SplitN(h, ":", 2)
		if len(kv) != 2 {
			logger.Printf("[error] invalid header %q, use \"key: this is the value\"", h)
			os.Exit(1)
		}
		header.Add(kv[0], strings.TrimSpace(kv[1]))
	}

	cfg := &target.Config{
		Stream:         *stream,
		NumAgents:      *numAgents,
		Throttle:       *qps,
		Pause:          *pause,
		MaxRequests:    *maxRequests,
		RequestTimeout: *requestTimeout,
		RunTimeout:     *runTimeout,
		Header:         header,
		Endpoint:       *endpoint,
		BodyConfig: &target.BodyConfig{
			NumErrors:       *numErrors,
			NumTransactions: *numTransactions,
			NumSpans:        *numSpans,
			NumFrames:       *numFrames,
		},
		DisableCompression: *disableCompression,
		DisableKeepAlives:  *disableKeepAlives,
		DisableRedirects:   *disableRedirects,
	}

	t := target.NewTargetFromConfig(*baseUrl, *method, cfg)
	work := t.GetWork(os.Stdout)

	start := time.Now()
	done := make(chan struct{})
	go func(w *requester.Work) {
		logger.Println("[info] starting worker for", t.Config.Endpoint)
		w.Run()
		logger.Println("[info] worker done for", t.Config.Endpoint)
		close(done)
	}(work)

	stopWorking := func() {
		go func(w *requester.Work) {
			logger.Println("[info] stopping worker for", t.Config.Endpoint)
			w.Stop()
		}(work)
	}

	// stop working on signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		logger.Println("[error] caught signal, stopping work")
		stopWorking()
	}()

	select {
	case <-done:
		logger.Println("[info] no more requests to make")
	case <-time.After(*runTimeout):
		logger.Println("[info] no more time left to make requests")
		stopWorking()
		select {
		case <-done:
			logger.Println("[info] stopped cleanly after time expired")
		case <-time.After(10 * time.Second):
			logger.Println("[error] failed to stop cleanly after timeout, aborting")
		}
	}

	out.PrintResults(work, time.Now().Sub(start).Seconds(), os.Stdout)
}
