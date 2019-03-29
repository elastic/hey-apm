package es

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/elastic/hey-apm/commands"
	"github.com/elastic/hey-apm/util"
	"github.com/olivere/elastic"
)

type es struct {
	*elastic.Client
	Url          string
	username     string
	password     string
	indexPattern string
	Err          error
}

// returns a client connected to an ElasticSearch node with given `params`
// "local" is short for http://localhost:9200
func elasticsearch(params ...string) es {
	url := util.Get(0, params)
	if url == "local" {
		url = "http://localhost:9200"
	}
	username := util.Get(1, params)
	password := util.Get(2, params)
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetBasicAuth(username, password),
		elastic.SetSniff(false),
	)
	es := es{client, url, username, password, "hey-bench", err}
	return es
}

type ReportNode es

func NewReportNode(params ...string) ReportNode {
	es := elasticsearch(params...)
	return ReportNode{es.Client, es.Url, es.username, es.password, "hey-bench", es.Err}
}

func (es ReportNode) FetchReports() ([]commands.TestReport, error) {
	ret := make([]commands.TestReport, 0)

	search, err := es.Search(es.indexPattern).
		Sort("@timestamp", false).
		Size(1000).
		Do(context.Background())
	if err != nil {
		return ret, err
	}

	for _, hit := range search.Hits.Hits {
		if err == nil {
			var report commands.TestReport
			err = json.Unmarshal(*hit.Source, &report)
			ret = append(ret, report)
		}
	}
	return ret, err
}

func (es ReportNode) IndexReport(r commands.TestReport) error {
	_, err := es.Index().
		Index(es.indexPattern).
		Type("_doc").
		BodyJson(r).
		Do(context.Background())
	es.Refresh(es.indexPattern).Do(context.Background())
	return err
}

type TestNode es

func NewTestNode(params ...string) TestNode {
	es := elasticsearch(params...)
	return TestNode{es.Client, es.Url, es.username, es.password, "apm-*", es.Err}
}

func (es TestNode) Count() int {
	if es.Err != nil {
		return -1
	}
	es.Flush(es.indexPattern).Do(context.Background())
	ret, err := es.Search(es.indexPattern).Do(context.Background())
	if err != nil {
		return -1
	} else {
		// exclude onboarding doc
		return int(math.Max(0, float64(ret.Hits.TotalHits-1)))
	}
}

func (es TestNode) Health() (string, error) {
	if es.Err != nil {
		return "", es.Err
	}
	status, err := es.ClusterHealth().Do(context.Background())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(status.Status), nil
}

// deletes all apm-* indices
func (es TestNode) Reset() error {
	if es.Err != nil {
		return es.Err
	}
	_, err := es.Client.DeleteIndex(es.indexPattern).Do(context.Background())
	if err == nil {
		_, err = es.Client.Flush(es.indexPattern).Do(context.Background())
	}
	return err
}
