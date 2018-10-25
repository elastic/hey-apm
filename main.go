package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/elastic/hey-apm/output"
	"github.com/elastic/hey-apm/server"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/target"
	"github.com/simitt/hey/requester"
)

var (
	// run options
	runTimeout  = flag.Duration("run", 30*time.Second, "stop run after this duration")
	maxRequests = flag.Int("requests", math.MaxInt32, "maximum requests to make")

	// payload options
	numAgents       = flag.Int("c", 1, "concurrent clients")
	qps             = flag.Float64("q", 0, "queries per second")
	numErrors       = flag.Int("e", 1, "number of errors")
	numFrames       = flag.Int("f", 1, "number of stacktrace frames per span")
	numSpans        = flag.Int("s", 2, "number of spans")
	numTransactions = flag.Int("t", 1, "number of transactions")

	// http options
	baseUrl            = flag.String("base-url", "http://localhost:8200", "")
	endpoint           = flag.String("endpoint", "/intake/v2/events", "")
	method             = flag.String("method", "POST", "method type")
	headers            = newStringsOpt("header", []string{}, "header(s) added to all requests")
	requestTimeout     = flag.Int("request-timeout", 30, "request timeout in seconds")
	idleTimeout        = flag.Duration("idle-timeout", 3*time.Minute, "idle timeout")
	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")

	describe    = flag.Bool("describe", false, "describe payloads and exit")
	dump        = flag.Bool("dump", false, "dump payloads in loadbeat config format and exit")
	interactive = flag.Bool("interactive", false, "run hey-apm in interactive mode listening on 8234")
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

func desc(t *target.Target) {
	//TODO: ensure body is properly handled
	var gzBody bytes.Buffer
	if len(t.Body) > 0 {
		zw := gzip.NewWriter(&gzBody)
		zw.Write(t.Body)
		zw.Close()
	}
	fmt.Printf("%s %s - %d (%d gz) bytes", t.Method, t.Url, len(t.Body), len(gzBody.Bytes()))

	var j map[string]interface{}
	if err := json.Unmarshal(t.Body, &j); err != nil {
		fmt.Println(err)
		return
	}
	var errs, trs, spans []interface{}
	for k, v := range j {
		switch k {
		case "error":
			errs = append(errs, v)
		case "transaction":
			trs = append(trs, v)
		case "span":
			spans = append(spans, v)
		default:
			fmt.Printf("Unknown type %s", k)
		}
	}
	fmt.Printf(" - %d errors", len(errs))
	fmt.Printf(" - %d transactions", len(trs))
	fmt.Printf(" - %d spans", len(spans))
	fmt.Println()
}

func dumpLoadbeat(t *target.Target) {
	//TODO: ensure body is properly decoded
	profileName := fmt.Sprintf("%s_%d_%d_%d_%d_%d_%d.yml", t.Method, *numErrors, *numTransactions, *numSpans, *numFrames, *numAgents, int(*qps))
	f, _ := os.Create(profileName)
	fmt.Fprintln(f, "loadbeat:")
	fmt.Fprintln(f, "  targets:")
	defer f.Close()
	fmt.Fprintf(f, "    - concurrent: %d\n", *numAgents)
	fmt.Fprintf(f, "      qps: %.5f\n", *qps)
	fmt.Fprintf(f, "      method: %s\n", t.Method)
	fmt.Fprintf(f, "      url: %s\n", t.Url)
	if len(t.Body) > 0 {
		fmt.Fprintln(f, "      headers:")
		fmt.Fprintln(f, "        - Content-Type:application/json")
		fmt.Fprintf(f, "      body: >\n        %s\n", t.Body)
	}
	fmt.Fprintln(f)
	return
}

func main() {
	flag.Parse()

	logger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

	if *interactive {
		io.BootstrapChecks()
		logger.Println("Starting hey-apm in interactive mode...")
		logger.Println("Connect with 'rlwrap telnet localhost 8234'")
		logger.Println("WARNING: Multiple concurrent tests against the same apm-server and/or elasticsearch instances will interfere with each other")
		server.Serve()
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
		Concurrent:     *numAgents,
		Qps:            *qps,
		MaxRequests:    *maxRequests,
		RequestTimeout: *requestTimeout,
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

	t := target.NewTarget(*baseUrl, *method, cfg)
	work := t.GetWork()

	if *describe {
		desc(t)
		return
	}
	if *dump {
		dumpLoadbeat(t)
		return
	}

	start := time.Now()
	done := make(chan struct{})
	go func(w *requester.Work) {
		logger.Println("[info] starting worker for", t.Url)
		w.Run()
		logger.Println("[info] worker done for", t.Url)
		close(done)
	}(work)

	stopWorking := func() {
		go func(w *requester.Work) {
			logger.Println("[info] stopping worker for", t.Url)
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

	output.PrintResults(work, time.Now().Sub(start).Seconds(), os.Stdout)
}
