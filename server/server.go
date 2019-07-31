package server

import (
	"encoding/json"
	errs "errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/es"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/strcoll"
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
}

type Memstats struct {
	TotalAlloc     int64 `json:"TotalAlloc"`
	HeapAlloc      int64 `json:"HeapAlloc"`
	Mallocs        int64 `json:"Mallocs"`
	NumGC          int64 `json:"NumGC"`
	TotalAllocDiff int64
	HeapAllocDiff  int64
}

// Sub subtracts some memory stats from another
func (ms Memstats) Sub(ms2 Memstats) Memstats {
	return Memstats{
		TotalAlloc:     ms.TotalAlloc,
		HeapAlloc:      ms.HeapAlloc,
		TotalAllocDiff: ms.TotalAlloc - ms2.TotalAlloc,
		HeapAllocDiff:  ms.HeapAlloc - ms2.HeapAlloc,
		Mallocs:        ms.Mallocs - ms2.Mallocs,
		NumGC:          ms.NumGC - ms2.NumGC,
	}
}

func (ms Memstats) String() string {
	metrics := strcoll.NewTuples()
	metrics.Add("heap", conv.ByteCountDecimal(ms.HeapAlloc))
	metrics.Add("total allocated", conv.ByteCountDecimal(ms.TotalAllocDiff))
	metrics.Add("heap allocated", conv.ByteCountDecimal(ms.HeapAllocDiff))
	metrics.Add("mallocs", ms.Mallocs)
	metrics.Add("num GC", ms.NumGC)
	return metrics.Format(20)
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
	for idx, arg := range cmd {
		switch {
		case arg == "-E":
			lookup = true
		case lookup:
			k, v := strcoll.SplitKV(cmd[idx], "=")
			if !strings.Contains(strings.ToLower(k), "password") {
				ret[k] = v
			}
			lookup = false
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
