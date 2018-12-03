package client

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/docker"
	"github.com/elastic/hey-apm/server/strcoll"
)

func (es es) Count() int64 {
	if es.useErr != nil {
		return -1
	}
	es.Flush("apm-*").Do(context.Background())
	ret, err := es.Search("apm-*").Do(context.Background())
	if err != nil {
		return -1
	} else {
		// exclude onboarding doc
		return int64(math.Max(0, float64(ret.Hits.TotalHits-1)))
	}
}

func (es es) Health() (string, error) {
	if es.useErr != nil {
		return "", es.useErr
	}
	status, err := es.ClusterHealth().Do(context.Background())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(status.Status), nil
}

func (es es) FetchReports() ([]api.TestReport, error) {
	ret := make([]api.TestReport, 0)

	search, err := es.Client.Search(es.reportIndex).
		Sort("@timestamp", false).
		Size(1000).
		Do(context.Background())
	if err != nil {
		return ret, err
	}

	for _, hit := range search.Hits.Hits {
		if err == nil {
			var report api.TestReport
			err = json.Unmarshal(*hit.Source, &report)
			ret = append(ret, report)
		}
	}
	return ret, err
}

func (es es) Url() string {
	return es.url
}

func (apm *apm) Log() []string {
	apm.mu.RLock()
	defer apm.mu.RUnlock()
	return apm.log
}

func (apm apm) IsRunning() *bool {
	if apm.isRemote {
		return nil
	}
	isRunning := apm.cmd != nil && apm.cmd.Process != nil && apm.cmd.ProcessState == nil
	if apm.isDockerized() {
		sh := io.Shell(io.NewBufferWriter(), docker.Dir(), false)
		out, err := sh("docker", "exec", "-i", docker.Container(), "ps", "-C", "apm-server", "h")
		isRunning = isRunning && err == nil && string(out) != ""
	}
	return &isRunning
}

func (apm apm) PrettyRevision() string {
	return apm.prettyRev
}

func (apm apm) Urls() []string {
	if apm.Dir() == "" {
		return apm.urls
	} else if apm.Dir() == "docker" {
		return []string{"http://0.0.0.0:8200"}
	}
	return []string{"http://localhost:8200"}
}

func (apm apm) isDockerized() bool {
	return apm.loc == "docker"
}

func (apm apm) Dir() string {
	return apm.loc
}

func (apm apm) Branch() string {
	return apm.branch
}

func (env evalEnvironment) ApmServer() api.ApmServer {
	return env.apm
}

func (env evalEnvironment) ElasticSearch() api.ElasticSearch {
	return env.es
}

func (env evalEnvironment) Ready() error {
	var err error
	if running := env.IsRunning(); running != nil && !*running {
		err = errors.New("apm server is not running\n")
	}
	if err == nil {
		_, err = env.Health()
	}
	return err
}

func (env evalEnvironment) Names() map[string][]string {
	return strcoll.Copy(env.nameDefs)
}
