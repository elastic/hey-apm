package server

import (
	"encoding/json"
	errs "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/strcoll"
)

type Info struct {
	BuildDate time.Time `json:"build_date"`
	BuildSha  string    `json:"build_sha"`
	Version   string    `json:"version"`
}

type ExpvarMetrics struct {
	Cmdline  []string `json:"cmdline"`
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

func QueryInfo(secret, url string) (Info, error) {
	body, err := request(secret, url)
	info := Info{}
	if err == nil {
		err = json.Unmarshal(body, &info)
	}
	return info, err
}

func QueryExpvar(secret, raw string) (ExpvarMetrics, error) {
	u, _ := url.Parse(raw)
	u.Path = "/debug/vars"
	body, err := request(secret, u.String())
	metrics := ExpvarMetrics{}
	if err == nil {
		err = json.Unmarshal(body, &metrics)
	}
	return metrics, err
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
