package api

import (
	"errors"
	"fmt"
	stdio "io"
	"math"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/strcoll"
)

type TestReport struct {
	/*
		metadata
	*/
	// for future multi-tenancy support, not used for now
	User string `json:"user"`
	// generated programmatically, useful to eg search all fields in elasticsearch
	ReportId string `json:"report_id"`
	// APM Server intake API version
	APIVersion string `json:"api_version"`
	// io.GITRFC
	ReportDate string `json:"report_date"`
	// host in which hey was running when generated this report, empty if `os.Hostname()` failed
	ReporterHost string `json:"reporter_host"`
	// git revision of hey-apm when generated this report, empty if `git rev-parse HEAD` failed
	ReporterRevision string `json:"reporter_revision"`
	// like reportDate, but better for querying ES and sorting
	Timestamp time.Time `json:"@timestamp"`
	// hardcoded to python for now
	Lang string `json:"language"`
	// any error (eg missing data) for which this report shouldn't be saved and considered for data analysis
	Error error

	// apmFlags passed to apm-server at startup
	ApmFlags string `json:"apm_flags"`
	// git revision hash
	Revision string `json:"revision"`
	// git revision date as io.GITRFC
	RevDate string `json:"rev_date"`
	// either maximum resident set size from a locally running process or from the containerized process
	MaxRss int64 `json:"max_rss"`
	// memory limit in bytes passed to docker, -1 if not applicable
	Limit int64 `json:"limit"`
	// size in bytes of a single request body, uncompressed
	ReqSize int `json:"request_size"`
	// holds all information available just after a test run
	TestResult
}

// methods in this type are not necessarily safe (eg: divide by 0, date parsing error, etc)
type TestResult struct {
	Cancelled bool `json:"cancelled"`
	/*
		independent variables
	*/
	// specified by the user, used for querying
	Duration time.Duration `json:"duration"`
	// actual elapsed time, used for X-per-second kind of metrics
	Elapsed time.Duration `json:"elapsed"`
	// errors per request
	Errors int `json:"errors"`
	// transactions per request
	Transactions int `json:"transactions"`
	// spans per transaction
	Spans int `json:"spans"`
	// frames per error and/or span
	Frames int `json:"frames"`
	// Number of concurrent agents sending requests.
	Agents int `json:"agents"`
	// queries per second cap, fixed to a very high number
	Throttle int `json:"throttle"`
	// size in bytes of a single request body, compressed
	GzipReqSize int `json:"gzip_request_size"`
	// request timeout in the agent
	ReqTimeout time.Duration `json:"request_timeout"`
	// whether it streams events or not
	Stream bool `json:"stream"`
	// includes protocol, hostname and port
	ElasticUrl string `json:"elastic_url"`
	// includes protocol, hostname and port
	// for now used to tell docker from from local process
	ApmUrl string `json:"apm_url"`
	// derived from apm_url
	ApmHost string `json:"apm_host"`
	// git branch
	Branch string `json:"branch"`
	/*
		dependent variables
	*/
	// total number of responses
	TotalResponses int `json:"total_responses"`
	// total number of accepted requests
	AcceptedResponses int `json:"accepted_responses"`
	// total number of elasticsearch docs indexed
	ActualDocs int64 `json:"actual_indexed_docs"`
	// number of elasticsearch docs encoded in the JSON body of the request
	DocsPerRequest int `json:"docs_per_request"`
	// milliseconds per accepted request
	Latency float64 `json:"latency_ms"`
	// number of requests per second
	PushedRps float64 `json:"pushed_rps"`
	// number of accepted requests per second
	AcceptedRps float64 `json:"accepted_rps"`
	// pushed volume per second, in bytes
	PushedBps float64 `json:"pushed_bps"`
	// accepted volume per second, in bytes
	AcceptedBps float64 `json:"accepted_bps"`
	// number of docs indexed per second
	Throughput float64 `json:"throughput"`
	// number of expected docs indexed after a run
	ExpectedDocs float64 `json:"expected_indexed_docs"`
	// ratio between indexed and expected docs
	// can be more than 1 if unexpected errors were returned (r/w timeouts, broken pipe, etc)
	ActualExpectRatio float64 `json:"actual_expected_ratio"`
	// how much memory takes to process some amount of data during 1 minute
	// eg: if memory used is 10mb and accepted volume in 1 minute is 2mb, this returns 0.2
	Efficiency float64 `json:"efficiency"`
}

// creates and validates a report out of a test result
func NewReport(result TestResult, usr, rev, revDate string, unstaged, isRemote bool, reqSize, mem, memLimit int64, flags []string, w stdio.Writer) TestReport {
	r := TestReport{
		Lang:       "python",
		APIVersion: "v2",
		ReportId:   randId(time.Now().Unix()),
		ReportDate: time.Now().Format(io.GITRFC),
		Timestamp:  time.Now(),
		User:       usr,
		Revision:   rev,
		RevDate:    revDate,
		MaxRss:     mem,
		Limit:      memLimit,
		ApmFlags:   s.Join(flags, " "),
		ReqSize:    int(reqSize),
		TestResult: result,
	}
	for _, check := range []struct {
		isOk     func() bool
		errMsg   string
		doEffect func()
	}{
		{
			isOk:   func() bool { return !r.Cancelled },
			errMsg: "test cancelled",
		},
		{
			isOk:   func() bool { return !isRemote },
			errMsg: "apm-server is not managed by hey-apm (an URL was provided in `apm use`)",
		},
		{
			isOk:   func() bool { return !unstaged },
			errMsg: "git reported unstaged changes",
		},
		{
			isOk:   func() bool { return r.Branch != "" },
			errMsg: "unknown branch",
			doEffect: func() {
				io.ReplyNL(w, fmt.Sprintf("\non branch %s", r.Branch))
			},
		},
		{
			isOk:   func() bool { return r.Revision != "" },
			errMsg: "unknown revision",
		},
		{
			isOk:   func() bool { return !r.revisionDate().IsZero() },
			errMsg: "unknown revision date",
		},
		{
			isOk:   func() bool { return r.Duration.Seconds() >= 30 },
			errMsg: "test duration too short",
		},
		{
			isOk: func() bool { return r.Elapsed.Seconds() > 0 },
			doEffect: func() {
				r.PushedRps = float64(r.TotalResponses) / r.Elapsed.Seconds()
				r.AcceptedRps = float64(r.AcceptedResponses) / r.Elapsed.Seconds()
				r.PushedBps = float64(r.ReqSize) * r.PushedRps
				r.AcceptedBps = float64(r.ReqSize) * r.AcceptedRps
				r.Throughput = float64(r.ActualDocs) / r.Elapsed.Seconds()

				io.ReplyNL(w, fmt.Sprintf("%spushed %s / sec , accepted %s / sec", io.Grey,
					byteCountDecimal(int64(r.PushedBps)),
					byteCountDecimal(int64(r.AcceptedBps))))
				io.ReplyNL(w, fmt.Sprintf("\n%s%d docs indexed (%.2f / sec)", io.Green,
					r.ActualDocs, r.Throughput))
			},
		},
		{
			isOk:   func() bool { return (r.Transactions+r.Errors) > 0 && r.AcceptedResponses > 0 },
			errMsg: "no accepted requests",
			doEffect: func() {
				r.Latency = 1000 / r.AcceptedRps
				r.ExpectedDocs = float64(r.AcceptedResponses) * float64(r.DocsPerRequest)
				r.ActualExpectRatio = float64(r.ActualDocs) / r.ExpectedDocs

				io.ReplyNL(w, fmt.Sprintf("%.2f%% of expected", 100*r.ActualExpectRatio))
				io.ReplyNL(w, fmt.Sprintf("%s%.2f ms / request", io.Green, r.Latency))
			},
		},
		{
			isOk:   func() bool { return r.MaxRss > 0 },
			errMsg: "memory usage not available",
			doEffect: func() {
				r.Efficiency = 60 * float64(r.AcceptedBps) / float64(r.MaxRss)

				io.ReplyNL(w, io.Green+byteCountDecimal(r.MaxRss)+" (max RSS)")
				io.ReplyNL(w, fmt.Sprintf("%s%.3f memory efficiency (accepted data volume per minute / memory used)",
					io.Green, r.Efficiency))

			},
		},
	} {
		if ok := check.isOk(); ok && check.doEffect != nil {
			check.doEffect()
		} else if !ok && r.Error == nil {
			r.Error = errors.New(check.errMsg)
		}
	}

	r.ReporterHost, _ = os.Hostname()
	selfDir := path.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/hey-apm")
	if rRev, err := io.Shell(io.NewBufferWriter(), selfDir, false)("git", "rev-parse", "HEAD"); err != nil {
		r.ReporterRevision = rRev
	}

	io.ReplyNL(w, io.Grey)

	return r
}

func (r TestReport) esHost() string {
	url, err := url.Parse(r.ElasticUrl)
	if err != nil {
		return r.ElasticUrl
	}
	return url.Hostname()
}

func apmHost(apmUrl string) string {
	url, err := url.Parse(apmUrl)
	if err != nil {
		return apmUrl
	}
	return url.Hostname()
}

func (r TestReport) apmFlags() []string {
	if len(r.ApmFlags) > 0 {
		return s.Split(r.ApmFlags, " ")
	}
	return nil
}

func (r TestReport) date() time.Time {
	t, _ := time.Parse(io.GITRFC, r.ReportDate)
	return t
}

func (r TestReport) revisionDate() time.Time {
	t, _ := time.Parse(io.GITRFC, r.RevDate)
	return t
}

// functions of this type map a subset of attribute names to their (stringified) values
// queries are performed against such maps
type data func(TestReport) map[string]string

// returns a map of attributes provided by the user, excluding apm-server flags and elastic host
// apm-server flags are considered separately, and elastic host is redundant
func independentVars(r TestReport) map[string]string {
	return map[string]string{
		// r.esHost() is an independent variable, but not queryable by the user
		// esHost() is always an implicit filter for each query
		"duration":        r.Duration.String(),
		"errors":          strconv.Itoa(r.Errors),
		"transactions":    strconv.Itoa(r.Transactions),
		"spans":           strconv.Itoa(r.Spans),
		"frames":          strconv.Itoa(r.Frames),
		"agents":          strconv.Itoa(r.Agents),
		"throttle":        strconv.Itoa(r.Throttle),
		"stream":          strconv.FormatBool(r.Stream),
		"request_timeout": r.ReqTimeout.String(),
		"revision":        r.Revision,
		"branch":          r.Branch,
		"apm_host":        r.ApmHost,
		"limit":           strconv.Itoa(int(r.Limit)),
	}
}

// converts "-E apm-server.flag=a" into {"apm-server.flag":"a"}
// only flags set by the user
func apmFlags(r TestReport) map[string]string {
	ret := make(map[string]string)
	var prev string
	for _, flag := range r.apmFlags() {
		if prev == "-E" &&
			!s.HasPrefix(flag, "apm-server.host") &&
			!s.HasPrefix(flag, "output.elasticsearch") {
			split := s.Split(flag, "=")
			if len(split) == 2 {
				ret[s.TrimSpace(split[0])] = s.TrimSpace(split[1])
			}
		}
		prev = flag
	}
	return ret
}

// attributes not set by the user that still makes sense to filter by
func metadata(r TestReport) map[string]string {
	return map[string]string{
		"report_id":     r.ReportId,
		"report_date":   r.ReportDate,
		"revision_date": r.RevDate,
		// not really metadata, but derived from independent variables
		"request_size": strconv.Itoa(r.ReqSize),
	}
}

type queryFilter struct {
	// field to filter by, eg "duration"
	k string
	// value to of the field to be matched, eg "1m"
	v string
	// comparison operator: =, !=, >, <
	op string
}

type query struct {
	// returns a subset of all the <attribute,value> pairs that the query filters will be matched against
	data data
	// expected size of the data, -1 if irrelevant
	size int
	// keys in query filters are expected to be a subset of keys in data
	filters []queryFilter
}

// parses a list of expressions like a=b, a!=b, a>b, a<b
func queryFilters(expressions []string) ([]queryFilter, error) {
	ret := make([]queryFilter, 0)
	var err error
	for idx, expr := range expressions {
		// order is relevant: "=" matches both "=" and "!="
		for _, op := range []string{"!=", "=", ">", "<"} {
			if parts := s.Split(expr, op); len(parts) == 2 {
				part0 := s.TrimSpace(parts[0])
				part1 := s.TrimSpace(parts[1])
				if part0 != "" && part1 != "" {
					ret = append(ret, queryFilter{part0, part1, op})
				}
				break
			}
		}
		if len(ret) < idx+1 && err == nil {
			err = errors.New(expr + " is not a valid filter, must use one of = != < >")
		}
	}
	return ret, err
}

// returns true if the report from the most recent revision shows no less efficient than reports from older revisions
// `filtersExpr` must include all the independent variables except revision and apm_host
// `all` must not be empty
func verify(since string, filtersExpr []string, all []TestReport) (bool, string, error) {
	if len(all) == 0 {
		return false, "", errors.New("no reports")
	}
	filters, err := queryFilters(filtersExpr)
	filterKeys := make([]string, 0)
	for _, filter := range filters {
		filterKeys = append(filterKeys, filter.k)
	}
	for k, _ := range independentVars(all[0]) {
		if k != "revision" && k != "apm_host" && !strcoll.Contains(k, filterKeys) {
			if k == "limit" {
				return false, "", errors.New("limit is a required filter:\n " +
					"for localhost tests is -1, default for dockerized tests is 4000000000 (in bytes)")
			}
			return false, "", errors.New(k + " is a required filter")
		}
	}
	reports, err := top(since, "revision_date", filters, all, err)
	if len(reports) == 0 {
		return false, "no data", err
	}
	best := best(reports)
	if best < 1 {
		return true, io.Green + "ok", err
	} else {
		last := reports[0]
		challenger := reports[best]
		return true, fmt.Sprintf("revision %s (%s) outperforms %s (%s): %.3f < %.3f\n"+
			"report ids: %s, %s (elasticsearch host = %s)",
			challenger.Revision, challenger.RevDate,
			last.Revision, last.RevDate,
			challenger.Efficiency, last.Efficiency,
			challenger.ReportId, last.ReportId,
			challenger.esHost()), err
	}

}

// applies the given filters to `all`, and returns up to `ND` reports sorted by `sortCriteria`
// then, for each report it finds up to 7 reports with the same test parameters values except for `variable`
// - see implementation for special cases regarding branch/revision and apm-server flags
// - ND might be a number or a duration (see `head`)
// - filters syntax and sortable fields are also described elsewhere
func collate(ND, variable, sortCriteria string, align bool, filtersExpr []string, all []TestReport) ([][][]string, error) {
	ret := make([][][]string, 0)
	// keep track of observed reports to avoid duplicated in results
	ids := make([]string, 0)
	var newReports []TestReport
	filters, err := queryFilters(filtersExpr)
	all = unique(all)
	reports, err := top(ND, sortCriteria, filters, all, err)
	for _, report := range reports {
		var variants []TestReport
		variants, err = values(findVariants(variable, report, all, err))
		variants, ids = seen(variants, ids)
		newReports, ids = seen([]TestReport{report}, ids)
		if len(newReports) == 0 {
			continue
		}
		// maybe a different sorting criteria would be better?
		variants, err = top("7", sortCriteria, nil, unique(variants), err)
		best := best(append(newReports, variants...))
		digestMatrix := make([][]string, len(variants)+3)
		digestMatrix[0] = digestMatrixHeader(variable, independentVars(report))
		digestMatrix[1], digestMatrix[2] = digest(report, variable, align, best == 0)
		for idx, variant := range variants {
			_, digestMatrix[3+idx] = digest(variant, variable, align, best == idx+1)
		}
		ret = append(ret, digestMatrix)
	}
	return ret, err
}

func top(ND, criteria string, filters []queryFilter, reports []TestReport, err error) ([]TestReport, error) {
	reports = sortBy(criteria, reports)
	query := query{combine(independentVars, apmFlags, metadata), -1, filters}
	ret, err := values(filter(query, reports, err))
	return head(ND, ret), err
}

// returns a subset of `bs` with the same independentVars as `a` except for `variable`, which must be different
// returned reports are keyed by their index in the original `bs` slice
func findVariants(variable string, a TestReport, bs []TestReport, err error) (map[int]TestReport, error) {
	filters := make([]queryFilter, 0)
	var data data
	if strcoll.Contains(variable, keysExcluding("revision", independentVars(a))) {
		data = independentVars
	} else {
		// consider apm server apmFlags only when comparing the same revision or a unknown attribute (eg flag)
		// in different revisions apmFlags might be different / non comparable
		data = combine(independentVars, apmFlags)
	}
	for k, v := range data(a) {
		// special case: if variable branch, variable revision as well
		if k == variable || (variable == "branch" && k == "revision") {
			filters = append(filters, queryFilter{k, v, "!="})
		} else {
			filters = append(filters, queryFilter{k, v, "="})
		}
	}
	query := query{data, len(data(a)), filters}
	return filter(query, bs, err)
}

// returns the `reports` matching the filters specified by the `query`
// filters are AND'ed
// returned reports are keyed by their index in the original slice
func filter(query query, reports []TestReport, err error) (map[int]TestReport, error) {
	ret := make(map[int]TestReport)
OuterLoop:
	for idx, report := range reports {
		data := query.data(report)
		if query.size != -1 && query.size != len(data) {
			// this happens when comparing reports with different (number of) flags
			// query.size -1 means that the output of this function is not meant for comparison
			continue
		}
		for _, filt := range query.filters {
			var match bool
			if v, ok := data[filt.k]; ok && err == nil {
				match, err = compare(v, filt.v, filt.op)
			}
			// ok will be false when querying a flag that is not present in the current report
			if !match {
				continue OuterLoop
			}
		}
		ret[idx] = report
	}
	return ret, err
}

// compares strings, integers, duration and dates
// supported operator are +, !=, > and <
// when comparing dates, the first operand should be a io.GITRFC (as per Git output)
// and the second one just YYYY-MM-DD, as is easier for users to enter
func compare(s1, s2 string, op string) (bool, error) {
	if i1, err1 := strconv.Atoi(s1); err1 == nil {
		i2, err2 := strconv.Atoi(s2)
		switch op {
		case "=":
			return i1 == i2, err2
		case "!=":
			return i1 != i2, err2
		case ">":
			return i1 > i2, err2
		case "<":
			return i1 < i2, err2
		}
	} else if t1, err1 := time.Parse(io.GITRFC, s1); err1 == nil {
		t2, err2 := time.Parse(io.HUMAN, s2)
		t1 = t1.UTC().Truncate(time.Hour * 24)
		t2 = t2.UTC()
		switch op {
		case "=":
			return t1.Equal(t2), err2
		case "!=":
			return !t1.Equal(t2), err2
		case ">":
			return t1.After(t2), err2
		case "<":
			return t2.After(t1), err2
		}
	} else if d1, err1 := time.ParseDuration(s1); err1 == nil {
		d2, err2 := time.ParseDuration(s2)
		switch op {
		case "=":
			return d1 == d2, err2
		case "!=":
			return d1 != d2, err2
		case ">":
			return d1 > d2, err2
		case "<":
			return d1 < d2, err2
		}
	} else if op == "=" {
		return s1 == s2, nil
	} else if op == "!=" {
		return s1 != s2, nil
	}
	return false, errors.New(fmt.Sprintf("comparator %s not valid with attribute %s", op, s1))
}

func sortBy(criteria string, reports []TestReport) []TestReport {
	switch criteria {
	case "report_date":
		sort.Sort(descByReportDate{reports})
	case "revision_date":
		sort.Sort(descByRevDate{reports})
	case "duration":
		sort.Sort(descByDuration{reports})
	case "pushed_volume":
		sort.Sort(descByPushedVolume{reports})
	case "actual_expected_ratio":
		sort.Sort(descByActualExpectedRatio{reports})
	case "latency":
		sort.Sort(ascByLatency{reports})
	case "throughput":
		sort.Sort(descByThroughput{reports})
	case "efficiency":
		sort.Sort(descByEfficiency{reports})
	}
	return reports
}

// if 2 reports have the same independent variables, return the one that showed better performance
// reports are sorted by their date, most recent first
func unique(reports []TestReport) []TestReport {
	return uniq(sortBy("report_date", reports))
}

func uniq(reports []TestReport) []TestReport {
	uniques := make([]TestReport, 0)
	if len(reports) == 0 {
		return uniques
	}
	first, rest := reports[0], reports[1:]
	variant, _ := findVariants("", first, rest, nil)
	isUnique := true
	for _, k := range keys(variant, true) {
		if first.Efficiency > variant[k].Efficiency {
			rest = append(rest[:k], rest[k+1:]...)
		} else {
			isUnique = false
		}
	}
	if isUnique {
		uniques = append(uniques, first)
	}
	return append(uniques, uniq(rest)...)
}

// returns the values of the given map in order
func values(m map[int]TestReport, err error) ([]TestReport, error) {
	ret := make([]TestReport, len(m))
	for idx, k := range keys(m, false) {
		ret[idx] = m[k]
	}
	return ret, err
}

// returns the keys of the given map in ascending or descending order
func keys(m map[int]TestReport, desc bool) []int {
	keys := make([]int, 0)
	for k := range m {
		keys = append(keys, k)
	}
	if desc {
		sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	} else {
		sort.Sort(sort.IntSlice(keys))
	}
	return keys
}

// return the first k reports
// ND might be a number (return up to N reports) or a duration (return reports dating back to now - D)
func head(ND string, reports []TestReport) []TestReport {
	if i, err := strconv.Atoi(ND); err == nil {
		return reports[:int(math.Min(float64(i), float64(len(reports))))]
	} else if d, err := time.ParseDuration(ND); err == nil {
		ret := make([]TestReport, 0)
		for _, report := range reports {
			if report.date().Add(d).After(time.Now()) {
				ret = append(ret, report)
			}
		}
		return ret
	} else {
		return reports
	}
}

// returns a subset of `reports` whose ids's are not contained in `ids`
// the returned `ids` have appended the ids's of such reports
func seen(reports []TestReport, ids []string) ([]TestReport, []string) {
	ret := make([]TestReport, 0)
	for _, report := range reports {
		if !strcoll.Contains(report.ReportId, ids) {
			ids = append(ids, report.ReportId)
			ret = append(ret, report)
		}
	}
	return ret, ids
}

// returns all the keys in the map except `exclude`
func keysExcluding(exclude string, m map[string]string) []string {
	ret := make([]string, 0)
	for k, _ := range m {
		if k != exclude {
			ret = append(ret, k)
		}
	}
	return ret
}

func digestMatrixHeader(variable string, m map[string]string) []string {
	ret := make([]string, 0)
	// always the same order
	for _, attr := range []string{"duration", "errors", "transactions", "spans", "frames", "agents", "branch"} {
		if variable != attr {
			ret = append(ret, io.Magenta+attr+" "+io.Grey+m[attr])
		}
	}
	// special case due to that a different branch entails a different revision
	if variable != "branch" && variable != "revision" {
		ret = append(ret, io.Magenta+"revision "+io.Grey+m["revision"])
	}
	return ret
}

// a digest describes the most informative data
// returns something printable
func digest(r TestReport, variable string, align, isBest bool) ([]string, []string) {
	header := []string{
		io.Magenta + "report id",
		io.Magenta + "revision date ",
		io.Magenta + "pushed   ",
		io.Magenta + "accepted  ",
		io.Magenta + "throughput",
		io.Magenta + "latency",
		io.Magenta + "index",
		io.Magenta + "max rss",
		io.Magenta + "effic",
	}
	color := io.Grey
	if isBest && align {
		color = io.Green
	}
	indexColor := io.Grey

	if r.ActualExpectRatio < 0.7 && align {
		indexColor = io.Red
	} else if r.ActualExpectRatio < 0.85 && align {
		indexColor = io.Yellow
	} else {
		indexColor = color
	}

	data := []string{
		color + r.ReportId,
		color + r.revisionDate().Format(io.SHORT),
		color + byteCountDecimal(int64(r.PushedBps)) + "ps",
		color + byteCountDecimal(int64(r.AcceptedBps)) + "ps",
		color + fmt.Sprintf("%.1fdps", r.Throughput),
		color + fmt.Sprintf("%.0fms", r.Latency),
		fmt.Sprintf("%s%.1f%%", indexColor, r.ActualExpectRatio*100),
		io.Grey + byteCountDecimal(r.MaxRss),
		fmt.Sprintf("%s%.3f", color, r.Efficiency),
	}

	if val, ok := combine(independentVars, apmFlags)(r)[variable]; ok {
		header = append(header, io.Magenta+variable)
		data = append(data, io.Magenta+val)
	}

	if align {
		for idx, val := range data {
			data[idx] = fit(val, len(header[idx]))
		}
	}

	// if variable is not a flag, show flags as last column
	if _, ok := independentVars(r)[variable]; ok {
		header = append(header, io.Magenta+"flags")
		data = append(data, color+mapToStr(apmFlags(r)))
	}

	return header, data
}

// combines all the functions in the argument list into one that returns the same report as calling them in order
func combine(fns ...data) data {
	return func(report TestReport) map[string]string {
		ms := make([]map[string]string, 0)
		for _, fn := range fns {
			ms = append(ms, fn(report))
		}
		return strcoll.Concat(ms...)
	}
}

// shamelessly stolen from http://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d b", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cb", float64(b)/float64(div), "kMGTPE"[exp])
}

// truncates or fills s with spaces so that it has a fixed length (used for visually aligning columns)
func fit(s string, len int) string {
	ret := make([]rune, len)
	var idx int
	var r rune
	// surely there should be a simpler implementation...
	for idx, r = range s {
		if idx < len {
			ret[idx] = r
		} else {
			break
		}
	}
	for idx2, _ := range ret {
		if idx2 > idx {
			ret[idx2] = []rune(" ")[0]
		}
	}
	return string(ret)
}

func mapToStr(m map[string]string) string {
	var ret string
	ks := make([]string, 0)
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		ret = ret + k + "=" + m[k] + " "
	}
	return ret
}

const MARGIN = 1.33

// returns the slice index of the best performing report efficiency wise, if significant
func best(reports []TestReport) int {
	if len(reports) < 2 {
		return -1
	}
	sorted := make([]TestReport, len(reports))
	copy(sorted, reports)
	sorted = sortBy("efficiency", sorted)
	if e := sorted[0].Efficiency; e > sorted[1].Efficiency*MARGIN {
		for idx, report := range reports {
			if report.Efficiency == e {
				return idx
			}
		}
	}
	return -1
}
