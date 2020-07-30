package server

import (
	"bytes"
	"encoding/json"
	errs "errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/es"
)

type Status struct {
	Metrics               *ExpvarMetrics
	SpanIndexCount        uint64
	TransactionIndexCount uint64
	ErrorIndexCount       uint64
}

// GetStatus returns apm-server info and memory stats, plus elasticsearch counts of apm documents.
func GetStatus(logger *log.Logger, secret, url string, connection es.Connection) Status {
	status := Status{}

	metrics, err := QueryExpvar(secret, url)
	if err == nil {
		status.Metrics = &metrics
	} else {
		logger.Println(err.Error())
	}
	status.SpanIndexCount = es.Count(connection, "apm*span*")
	status.TransactionIndexCount = es.Count(connection, "apm*transaction*")
	status.ErrorIndexCount = es.Count(connection, "apm*error*")
	return status
}

type Info struct {
	BuildDate time.Time `json:"build_date"`
	BuildSha  string    `json:"build_sha"`
	Version   string    `json:"version"`
}

type Cmdline []string

type ExpvarMetrics struct {
	Cmdline  Cmdline  `json:"cmdline"`
	Memstats Memstats `json:"memstats"`
	LibbeatMetrics
}

type LibbeatMetrics struct {
	OutputEventsActive   *int64 `json:"libbeat.output.events.active"`
	PipelineEventsActive *int64 `json:"libbeat.pipeline.events.active"`
}

type Memstats struct {
	TotalAlloc     uint64 `json:"TotalAlloc"`
	HeapAlloc      uint64 `json:"HeapAlloc"`
	Mallocs        uint64 `json:"Mallocs"`
	NumGC          uint64 `json:"NumGC"`
	TotalAllocDiff uint64
}

// Sub subtracts some memory stats from another
func (ms Memstats) Sub(ms2 Memstats) Memstats {
	return Memstats{
		TotalAlloc:     ms.TotalAlloc,
		HeapAlloc:      ms.HeapAlloc,
		TotalAllocDiff: ms.TotalAlloc - ms2.TotalAlloc,
		Mallocs:        ms.Mallocs - ms2.Mallocs,
		NumGC:          ms.NumGC - ms2.NumGC,
	}
}

func (ms Memstats) String() string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 20, 8, 0, '.', 0)
	fmt.Fprintf(tw, "heap \t %s\n", humanize.Bytes(ms.HeapAlloc))
	fmt.Fprintf(tw, "total allocated \t %s\n", humanize.Bytes(ms.TotalAllocDiff))
	fmt.Fprintf(tw, "mallocs \t %d\n", ms.Mallocs)
	fmt.Fprintf(tw, "num GC \t %d\n", ms.NumGC)
	tw.Flush()
	return buf.String()
}

func (info Info) String() string {
	if info.Version == "" {
		return "unknown apm-server version"
	}
	return fmt.Sprintf("apm-server version %s built on %d %s [%s]",
		info.Version, info.BuildDate.Day(), info.BuildDate.Month().String(), info.BuildSha[:7])
}

// Parse returns all the -E arguments passed to an apm-server except passwords
func (cmd Cmdline) Parse() map[string]string {
	ret := make(map[string]string)
	var lookup bool
	for _, arg := range cmd {
		switch {
		case arg == "-E":
			lookup = true
		case lookup:
			lookup = false
			sep := strings.IndexRune(arg, '=')
			if sep < 0 {
				continue
			}
			k, v := arg[:sep], arg[sep+1:]
			if !strings.Contains(strings.ToLower(k), "password") {
				ret[k] = v
			}
		}
	}
	return ret
}

// QueryInfo sends a request to an apm-server health-check endpoint and parses the result.
func QueryInfo(secret, url string) (Info, error) {
	body, err := request(secret, url)
	info := Info{}
	if err == nil {
		err = json.Unmarshal(body, &info)
	}
	return info, err
}

// QueryExpvar sends a request to an apm-server /debug/vars endpoint and parses the result.
func QueryExpvar(secret, raw string) (ExpvarMetrics, error) {
	u, _ := url.Parse(raw)
	u.Path = "/debug/vars"
	body, err := request(secret, u.String())
	metrics := ExpvarMetrics{}
	if err == nil {
		err = json.Unmarshal(body, &metrics)
	}
	return metrics, errors.Wrap(err, fmt.Sprintf("error querying %s, ensure to start apm-server"+
		" with -E apm-server.expvar.enabled=true", u.Path))
}

func request(secret, url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	if secret != "" {
		req.Header.Set("Authorization", "Beater "+secret)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return body, errs.New("server status not OK: " + resp.Status)
	}
	return body, err
}
