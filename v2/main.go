package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/elastic/hey-apm/compose"
)

const ()

var (
	runTimeout         = flag.Duration("run", 10*time.Second, "stop run after this duration")
	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")
	numSpans           = flag.Int("s", 1, "number of spans")
	numFrames          = flag.Int("f", 1, "number of stacktrace frames per span")
	numAgents          = flag.Int("c", 3, "concurrent clients")

	restDuration = flag.Duration("rest-duration", 100*time.Millisecond, "how long to stop sending data")
	restInterval = flag.Duration("rest-interval", 500*time.Millisecond, "how often to stop sending data")
)

func do(parent context.Context, logger *log.Logger, client *http.Client, payloads [][]byte) {
	ctx, cancel := context.WithCancel(parent)
	reader, writer := io.Pipe()

	doneWriting := make(chan struct{})
	writes := 0 // payloads
	wrote := 0  // bytes
	go func(w io.WriteCloser) {
		defer close(doneWriting)
		if n, err := w.Write(addNewline(compose.Metadata)); err != nil {
			logger.Println("[error] writing metadata: ", err)
			return
		} else {
			writes++
			wrote += n
		}
		rest := time.After(*restInterval)
		for {
			for _, p := range payloads {
				select {
				case <-ctx.Done():
					//logger.Println("[debug] all done")
					w.Close()
					return
				case <-rest:
					time.Sleep(*restDuration)
					rest = time.After(*restInterval)
				default:
					if n, err := w.Write(p); err != nil {
						logger.Println("[error] writing payload: ", err)
						return
					} else {
						writes++
						wrote += n
					}
				}
			}
		}
	}(writer)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8200/v2/intake", reader)
	if err != nil {
		logger.Println("[error] creating request:", err)
		return
	}
	req.Header.Add("Content-Type", "application/ndjson")
	rsp, err := client.Do(req)
	cancel()
	if err != nil {
		logger.Println("[error] http client:", err)
		return
	}
	<-doneWriting
	rspBody, _ := ioutil.ReadAll(rsp.Body)
	logger.Printf("[info] after %d writes totaling %d bytes: %s %s\n", writes, wrote, rsp.Status, string(rspBody))
}

func main() {
	flag.Parse()
	ctx, _ := context.WithTimeout(context.Background(), *runTimeout)
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    10 * time.Minute,
		DisableCompression: *disableCompression,
		DisableKeepAlives:  *disableKeepAlives,
	}
	client := &http.Client{Transport: tr}
	if *disableRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	span := addNewline(compose.NdjsonWrapObj("span", compose.V2Span(*numFrames)))
	payloads := make([][]byte, 1+*numSpans)
	payloads[0] = addNewline(compose.NdjsonWrapObj("transaction", compose.SingleTransaction))
	for i := 1; i <= *numSpans; i++ {
		payloads[i] = span
	}

	var wg sync.WaitGroup
	wg.Add(*numAgents)
	for i := 0; i < *numAgents; i++ {
		logger := log.New(os.Stderr, fmt.Sprintf("[agent %d] ", i), log.Ldate|log.Ltime|log.Lshortfile)

		go func() {
			do(ctx, logger, client, payloads)
			wg.Done()
		}()
	}
	wg.Wait()
}

func addNewline(p []byte) []byte {
	return append(p, '\n')
}
