package main

import (
	"flag"
	"fmt"
	"log"
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
	timeout            = flag.Int("timeout", 3, "request timeout")
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
			RequestTimeout:     *timeout,
			DisableCompression: *disableCompression,
			DisableKeepAlives:  *disableKeepAlives,
			DisableRedirects:   *disableRedirects,
		}) {
			work = append(work, w)
		}
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
