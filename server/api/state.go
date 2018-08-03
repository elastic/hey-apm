package api

type ApmServer interface {
	Url() string
	Dir() string
	Branch() string
	PrettyRevision() string
	Log() []string
	IsRunning() *bool
}

type ElasticSearch interface {
	Count() int64
	Health() (string, error)
	FetchReports() ([]TestReport, error)
	Url() string
}

type State interface {
	ElasticSearch() ElasticSearch
	ApmServer() ApmServer
	Ready() error
	Names() map[string][]string
}
