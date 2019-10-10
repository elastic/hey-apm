package es

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v7/esutil"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/hey-apm/models"
	"github.com/elastic/hey-apm/strcoll"
	"github.com/pkg/errors"
)

const (
	reportingIndex = "hey-bench"
	local          = "http://localhost:9200"
)

// Connection holds an elasticsearch client plus URL and credentials strings
type Connection struct {
	*elasticsearch.Client
	Url      string
	username string
	password string
}

// NewConnection returns a client connected to an ElasticSearch node with given `params`
// "local" is short for http://localhost:9200
func NewConnection(url, auth string) (Connection, error) {
	if url == "local" {
		url = local
	}
	username, password := strcoll.SplitKV(auth, ":")

	cfg := elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
	}

	client, err := elasticsearch.NewClient(cfg)
	return Connection{client, url, username, password}, err
}

// IndexReport saves in elasticsearch a performance report.
func IndexReport(conn Connection, report models.Report) error {
	resp, err := conn.Index(reportingIndex, esutil.NewJSONReader(report),
		conn.Index.WithRefresh("true"),
		conn.Index.WithDocumentID(report.ReportId),
	)
	if err != nil {
		return err
	}

	if resp.IsError() {
		return errors.New(resp.String())
	}
	return nil
}

// FetchReports retrieves performance reports from elasticsearch.
func FetchReports(conn Connection, body interface{}) ([]models.Report, error) {
	resp, err := conn.Search(
		conn.Search.WithIndex(reportingIndex),
		conn.Search.WithSort("@timestamp:desc"),
		conn.Search.WithBody(esutil.NewJSONReader(body)),
	)
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, errors.New(resp.String())
	}

	parsed := SearchResult{}
	err = json.NewDecoder(resp.Body).Decode(&parsed)

	ret := make([]models.Report, len(parsed.Hits.Hits))

	for idx, hit := range parsed.Hits.Hits {
		hit.Source.ReportId = hit.Id
		ret[idx] = hit.Source
	}

	return ret, err
}

// Count returns the number of documents in the given index.
func Count(conn Connection, index string) uint64 {
	res, err := conn.Count(
		conn.Count.WithIndex(index),
	)
	if err != nil {
		return 0
	}
	var m map[string]interface{}
	json.NewDecoder(res.Body).Decode(&m)
	if ct, ok := m["count"]; ok && ct != nil {
		return uint64(m["count"].(float64))
	}
	return 0
}

func Delete(conn Connection, indices ...string) error {
	resp, err := conn.Indices.Delete(indices)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return errors.New(fmt.Sprintf("%s: %s", resp.Status(), resp.String()))
	}
	return nil
}
