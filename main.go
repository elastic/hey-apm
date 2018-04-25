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
	"sync"
	"time"

	"github.com/elastic/hey-apm/profile"
	"github.com/elastic/hey-apm/target"
	"github.com/graphaelli/hey/requester"
	"github.com/elastic/hey-apm/output"
)

var (
	baseUrls    = newStringsOpt("base-url", []string{"http://localhost:8200/"}, "")
	headers     = newStringsOpt("header", []string{}, "header(s) added to all requests")
	profileName = flag.String("profile", "heavy",
		fmt.Sprintf("load profile: one of %s", profile.Choices()))
	runTimeout         = flag.Duration("run", 10*time.Second, "stop run after this duration")
	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")
	maxRequests        = flag.Int("requests", math.MaxInt32, "maximum requests to make")
	timeout            = flag.Int("timeout", 3, "request timeout")
	describe           = flag.Bool("describe", false, "describe payloads and exit")
	dump               = flag.Bool("dump", false, "dump payloads in loadbeat config format and exit")
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

func dumpLoadbeat(profileName string, work []*requester.Work) {
	f, _ := os.Create(fmt.Sprintf("%s.yml", profileName))
	fmt.Fprintln(f, "loadbeat:")
	fmt.Fprintln(f, "  targets:")
	defer f.Close()
	for _, w := range work {
		fmt.Fprintf(f, "    - concurrent: %d\n", w.C)
		fmt.Fprintf(f, "      qps: %.5f\n", w.QPS)
		fmt.Fprintf(f, "      method: %s\n", w.Request.Method)
		fmt.Fprintf(f, "      url: %s\n", w.Request.URL)
		if len(w.RequestBody) > 0 {
			fmt.Fprintln(f, "      headers:")
			fmt.Fprintln(f, "        - Content-Type:application/json")
			fmt.Fprintf(f, "      body: >\n        %s\n", w.RequestBody)
		}
		fmt.Fprintln(f)
	}
}

func desc(work []*requester.Work) {
	for _, w := range work {
		var gzBody bytes.Buffer
		if len(w.RequestBody) > 0 {
			zw := gzip.NewWriter(&gzBody)
			zw.Write(w.RequestBody)
			zw.Close()
		}
		fmt.Printf("%s %s - %d (%d gz) bytes", w.Request.Method, w.Request.URL, len(w.RequestBody), len(gzBody.Bytes()))

		var j map[string]interface{}
		if err := json.Unmarshal(w.RequestBody, &j); err != nil {
			fmt.Println()
			continue
		}
		if ts := j["transactions"]; ts != nil {
			tsList := ts.([]interface{})
			fmt.Printf(" - %d transactions", len(tsList))
			if len(tsList) > 0 {
				fmt.Print(" with spans of length: ")
			}
			for _, t := range tsList {
				fmt.Print(len(t.(map[string]interface{})["spans"].([]interface{})), " ")
			}
		}
		fmt.Println()
	}
}

func main() {
	flag.Parse()

	logger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

	targets := profile.Get(*profileName)
	if len(targets) == 0 {
		logger.Println("[error] unknown profile")
		os.Exit(1)
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
		MaxRequests:        *maxRequests,
		RequestTimeout:     *timeout,
		DisableCompression: *disableCompression,
		DisableKeepAlives:  *disableKeepAlives,
		DisableRedirects:   *disableRedirects,
		Header:             header,
	}
	var work []*requester.Work
	for _, baseUrl := range baseUrls.List() {
		for _, w := range targets.GetWork(baseUrl, cfg) {
			work = append(work, w)
		}
	}

	if *describe {
		desc(work)
		return
	}
	if *dump {
		dumpLoadbeat(*profileName, work)
		return
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(len(work))
	for _, w := range work {
		go func(w *requester.Work) {
			logger.Println("[info] starting worker for", w.Request.URL)
			w.Run()
			logger.Println("[info] worker done for", w.Request.URL)
			wg.Done()
		}(w)
	}

	stopWorking := func() {
		for _, w := range work {
			go func(w *requester.Work) {
				logger.Println("[info] stopping worker for", w.Request.URL)
				w.Stop()
			}(w)
		}
	}

	// stop working on signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		logger.Println("[error] caught signal, stopping work")
		stopWorking()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
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
