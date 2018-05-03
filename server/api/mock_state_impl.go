package api

type MockApm struct {
	url, dir, branch string
	running          bool
	L                []string
}

func (a MockApm) Url() string {
	return a.url
}

func (a MockApm) Dir() string {
	return a.dir
}

func (a MockApm) Branch() string {
	return a.branch
}

func (a MockApm) PrettyRevision() string {
	return ""
}

func (a MockApm) Log() []string {
	return a.L
}

func (a MockApm) IsRunning() bool {
	return a.running
}

func (a MockApm) Error() error {
	return nil
}

type MockEs struct {
	docs        int64
	health, url string
	HealthErr   error
}

func (e MockEs) Count() int64 {
	return e.docs
}

func (e MockEs) Health() (string, error) {
	return e.health, e.HealthErr
}

func (e MockEs) FetchReports() ([]TestReport, error) {
	return nil, nil
}

func (e MockEs) Url() string {
	return e.url
}

type MockState struct {
	MockApm
	es MockEs
	Ok error
}

func (s MockState) ElasticSearch() ElasticSearch {
	return s.es
}

func (s MockState) ApmServer() ApmServer {
	return s
}

func (s MockState) Ready() error {
	return s.Ok
}

func (s MockState) Names() map[string][]string {
	return nil
}
