package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/elastic/hey-apm/profile"
	"github.com/elastic/hey-apm/target"
	"github.com/graphaelli/hey/requester"
)

var (
	baseUrls    = newStringsOpt("base-url", []string{"http://localhost:8200/"}, "")
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

// sortedTotal sorts the keys and sums the values of the input map
func sortedTotal(m map[int]int) ([]int, int) {
	keys := make([]int, len(m))
	i := 0
	total := 0
	for k, v := range m {
		keys[i] = k
		total += v
		i++
	}
	sort.Ints(keys)
	return keys, total
}

// errorCount is for sorting errors by count
type errorCount struct {
	err   string
	value int
}
type errorCountSlice []errorCount

func (e errorCountSlice) Len() int           { return len(e) }
func (e errorCountSlice) Less(i, j int) bool { return e[i].value < e[j].value }
func (e errorCountSlice) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }

// sortedTotalErrors sorts by the values of the input
func sortedErrors(m map[string]int) []string {
	errorCounts := make(errorCountSlice, len(m))
	i := 0
	for k, v := range m {
		errorCounts[i] = errorCount{err: k, value: v}
		i++
	}
	sort.Sort(sort.Reverse(errorCounts))
	keys := make([]string, len(errorCounts))
	for i, e := range errorCounts {
		keys[i] = e.err
	}
	return keys
}

// collapseError makes groups of similar errors identical
func collapseError(e string) string {
	// Post http://localhost:8201/v1/transactions: read tcp 127.0.0.1:63204->127.0.0.1:8201: read: connection reset by peer
	if strings.HasSuffix(e, "read: connection reset by peer") {
		return "read: connection reset by peer"
	}

	// Post http://localhost:8200/v1/transactions: net/http: HTTP/1.x transport connection broken: write tcp [::1]:63967->[::1]:8200: write: broken pipe
	if strings.HasSuffix(e, "write: broken pipe") {
		return "write: broken pipe"
	}
	return e
}

func printResults(work []*requester.Work, dur float64) {
	for i, w := range work {
		if i > 0 {
			fmt.Println()
		}

		statusCodeDist := w.StatusCodes()
		codes, total := sortedTotal(statusCodeDist)
		div := float64(total)
		fmt.Println(w.Request.URL, i)
		for _, code := range codes {
			cnt := statusCodeDist[code]
			fmt.Printf("  [%d]\t%d responses (%.2f%%) \n", code, cnt, 100*float64(cnt)/div)
		}
		fmt.Printf("  total\t%d responses (%.2f rps)\n", total, div/dur)

		errorTotal := 0
		errorDist := make(map[string]int)
		for err, num := range w.ErrorDist() {
			err = collapseError(err)
			errorDist[err] += num
			errorTotal += num
		}

		if errorTotal > 0 {
			errorKeys := sortedErrors(errorDist)
			fmt.Printf("\n  %d errors:\n", errorTotal)
			for _, err := range errorKeys {
				num := errorDist[err]
				fmt.Printf("  [%d]\t%s\n", num, err)
			}
		}
	}
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

	var work []*requester.Work
	for _, baseUrl := range baseUrls.List() {
		for _, w := range targets.GetWork(baseUrl, &target.Config{
			MaxRequests:        *maxRequests,
			RequestTimeout:     *timeout,
			DisableCompression: *disableCompression,
			DisableKeepAlives:  *disableKeepAlives,
			DisableRedirects:   *disableRedirects,
		}) {
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

	printResults(work, time.Now().Sub(start).Seconds())
}
