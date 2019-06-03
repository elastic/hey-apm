package es

import (
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v7/esutil"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/hey-apm/reports"
	"github.com/elastic/hey-apm/strcoll"
	"github.com/pkg/errors"
)

type Connection struct {
	*elasticsearch.Client
	Url      string
	username string
	password string
	Err      error
}

// returns a client connected to an ElasticSearch node with given `params`
// "local" is short for http://localhost:9200
func NewConnection(url, auth string) Connection {
	if url == "local" {
		url = "http://localhost:9200"
	}
	username, password := strcoll.SplitKV(auth, ":")

	cfg := elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
	}

	client, err := elasticsearch.NewClient(cfg)
	return Connection{client, url, username, password, err}
}

func FetchReports(conn Connection, index string) ([]reports.Report, error) {
	ret := make([]reports.Report, 0)
	res, err := conn.Search(
		conn.Search.WithIndex(index),
		conn.Search.WithSort("@timestamp:desc"),
	)
	parsed := SearchResult{}
	json.NewDecoder(res.Body).Decode(&parsed)
	for _, hit := range parsed.Hits.Hits {
		ret = append(ret, hit.Source)
	}
	return ret, err
}

func IndexReport(conn Connection, index string, report reports.Report) error {
	if conn.Err != nil {
		return conn.Err
	}
	res, err := conn.Index(index, esutil.NewJSONReader(report),
		conn.Index.WithRefresh("true"),
		conn.Index.WithDocumentID(report.ReportId),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return errors.New(res.String())
	}
	return nil
}

func Count(conn Connection, index string) uint64 {
	if conn.Err != nil {
		return 0
	}
	res, err := conn.Count(
		conn.Count.WithIndex(index),
	)
	if err != nil {
		return 0
	}
	var m map[string]interface{}
	json.NewDecoder(res.Body).Decode(&m)
	return uint64(m["count"].(float64))
}
