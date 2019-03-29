package commands

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/util"
)

// functions of this type map a subset of attribute names to their (stringified) values
// queries are performed against such maps
type data func(TestReport) map[string]string

// returns a map of attributes provided by the user, excluding labels
func inputAttributes(r TestReport) map[string]string {
	return map[string]string{
		// "labels":       r.Labels,
		"duration":     r.Duration.String(),
		"errors":       strconv.Itoa(r.Errors),
		"transactions": strconv.Itoa(r.Transactions),
		"spans":        strconv.Itoa(r.Spans),
		"frames":       strconv.Itoa(r.Frames),
		"agents":       strconv.Itoa(r.Agents),
		"throttle":     strconv.Itoa(r.Throttle),
		"stream":       strconv.FormatBool(r.Stream),
		"timeout":      r.ReqTimeout.String(),
		"branch":       r.ApmVersion,
		"apm_host":     r.ApmHost,
		"apms":         strconv.Itoa(r.NumApm),
		"es_host":      r.ElasticHost,
	}
}

// attributes that makes sense to filter by, but not breakdown by
func metadata(r TestReport) map[string]string {
	return map[string]string{
		"report_id":   r.ReportId,
		"report_date": r.ReportDate,
		// add apm-server build date
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
// `filtersExpr` must include all the independent variables except apm_host and apms
// `all` must not be empty
// TODO needs build_date
func verify(since time.Duration, filtersExpr []string, all []TestReport) (bool, error) {
	if len(all) == 0 {
		return false, errors.New("no reports")
	}
	filters, err := queryFilters(filtersExpr)
	filterKeys := make([]string, 0)
	for _, filter := range filters {
		filterKeys = append(filterKeys, filter.k)
	}
	for k, _ := range inputAttributes(all[0]) {
		if k != "apm_host" && k != "apms" && !util.Contains(k, filterKeys) {
			return false, errors.New(k + " is a required filter")
		}
	}
	reports, err := top(since, 100, "build_date", filters, all, err)
	if len(reports) == 0 {
		return false, err
	}
	best := best(reports)
	if best < 1 {
		return true, err
	} else {
		// last := reports[0]
		// challenger := reports[best]
		return true, err
	}

}

// applies the given filters to `all`, and returns up to `ND` reports sorted by `sortCriteria`
// then, for each report it finds up to 7 reports with the same test parameters values except for `variable`
// - see implementation for special cases regarding branch/revision and apm-server flags
// - ND might be a number or a duration (see `head`)
// - filters syntax and sortable fields are also described elsewhere
func collate(since time.Duration, size int, attribute, sortCriteria string, align bool, filtersExpr []string, all []TestReport) ([][][]string, error) {
	ret := make([][][]string, 0)
	// keep track of observed reports to avoid duplicated in results
	ids := make([]string, 0)
	var newReports []TestReport
	filters, err := queryFilters(filtersExpr)
	all = unique(all)
	reports, err := top(since, size, sortCriteria, filters, all, err)
	for _, report := range reports {
		var variants []TestReport
		variants, err = values(findVariants(attribute, report, all, err))
		variants, ids = seen(variants, ids)
		newReports, ids = seen([]TestReport{report}, ids)
		if len(newReports) == 0 {
			continue
		}
		// maybe a different sorting criteria would be better?
		variants, err = top(since, size, sortCriteria, nil, unique(variants), err)
		// best := best(append(newReports, variants...))
		digestMatrix := make([][]string, len(variants)+3)
		// digestMatrix[0] = digestMatrixHeader(attribute, inputAttributes(report))
		//digestMatrix[0], digestMatrix[1] = digest(report, attribute, align, best == 0)
		//for idx, variant := range variants {
		//	_, digestMatrix[3+idx] = digest(variant, attribute, align, best == idx+1)
		//}
		ret = append(ret, digestMatrix)
	}
	return ret, err
}

func top(since time.Duration, size int, criteria string, filters []queryFilter, reports []TestReport, err error) ([]TestReport, error) {
	reports = sortBy(criteria, reports)
	query := query{combine(inputAttributes, labelsMap, metadata), -1, filters}
	ret, err := values(filter(query, reports, err))
	return head(since, size, ret), err
}

// converts "-E apm-server.flag=a" into {"apm-server.flag":"a"}
// only flags set by the user
func labelsMap(r TestReport) map[string]string {
	ret := make(map[string]string)
	for _, label := range r.labels() {
		split := s.Split(label, "=")
		ret[s.TrimSpace(split[0])] = s.TrimSpace(split[1])
	}
	return ret
}

// returns a subset of `bs` with the same inputAttributes values as `a` except for `variable`, which must be different
// returned reports are keyed by their index in the original `bs` slice
func findVariants(attribute string, a TestReport, bs []TestReport, err error) (map[int]TestReport, error) {
	filters := make([]queryFilter, 0)
	var data data
	if util.Contains(attribute, keysExcluding([]string{"revision", "apm_host"}, inputAttributes(a))) {
		data = inputAttributes
	} else {
		// consider apm server labels only when comparing the same revision or a unknown attribute (eg flag)
		// in different revisions labels might be different / non comparable
		data = combine(inputAttributes, labelsMap)
	}
	for k, v := range data(a) {
		// special cases: branch and apms
		// ie. 2 results with different number of apms can't have the same apm_host value,
		// and 2 results with different branch can't have the same Git revision
		if k == attribute || (attribute == "branch" && k == "revision") || (attribute == "apms" && k == "apm_host") {
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
		// todo still needed?
		// if query.size != -1 && query.size != len(data) {
		// this happens when comparing reports with different (number of) flags
		// query.size -1 means that the output of this function is not meant for comparison
		// continue
		//}
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
	} else if t1, err1 := time.Parse(util.GITRFC, s1); err1 == nil {
		t2, err2 := time.Parse(util.HUMAN, s2)
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
	// case "build_date":
	//	sort.Sort(descByRevDate{reports})
	case "duration":
		sort.Sort(descByDuration{reports})
	case "pushed_volume":
		sort.Sort(descByPushedVolume{reports})
	case "throughput":
		sort.Sort(descByThroughput{reports})
	}
	return reports
}

// if 2 reports have the same independent variables, return the one that showed better performance
// reports are sorted by their date, most recent first
// performance is given by the efficiency variable, if that is unknown, then throughput is used instead
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
		if first.Throughput > variant[k].Throughput {
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

// return the first k reports
func head(since time.Duration, size int, reports []TestReport) []TestReport {
	ret := make([]TestReport, 0)
	for _, report := range reports {
		if report.date().Add(since).After(time.Now()) {
			ret = append(ret, report)
		}
	}
	return ret[:int(math.Min(float64(size), float64(len(ret))))]
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

// returns a subset of `reports` whose ids's are not contained in `ids`
// the returned `ids` have appended the ids's of such reports
func seen(reports []TestReport, ids []string) ([]TestReport, []string) {
	ret := make([]TestReport, 0)
	for _, report := range reports {
		if !util.Contains(report.ReportId, ids) {
			ids = append(ids, report.ReportId)
			ret = append(ret, report)
		}
	}
	return ret, ids
}

// returns all the keys in the map except `exclude`
func keysExcluding(exclude []string, m map[string]string) []string {
	ret := make([]string, 0)
	for k, _ := range m {
		if !util.Contains(k, exclude) {
			ret = append(ret, k)
		}
	}
	return ret
}

//func digestMatrixHeader(variable string, m map[string]string) []string {
//	ret := make([]string, 0)
//	// always the same order
//	for _, attr := range []string{"label", "duration", "errors", "transactions", "spans", "frames", "stream", "agents", "throttle", "branch"} {
//		if variable != attr {
//			ret = append(ret, io.Magenta+attr+" "+io.Grey+m[attr])
//		}
//	}
//	// special case due to that a different branch entails a different revision
//	if variable != "branch" && variable != "revision" {
//		ret = append(ret, io.Magenta+"revision "+io.Grey+m["revision"])
//	}
//	// special case due to that a different number of apms entails a different apm_host string
//	if variable != "apms" && variable != "apm_host" {
//		ret = append(ret, io.Magenta+"apm_host "+io.Grey+m["apm_host"])
//	}
//	return ret
//}

//// a digest describes the most informative performance data
//func digest(r TestReport, variable string, align, isBest bool) ([]string, []string) {
//	//header := []string{
//	//	"pushed   ",
//	//	"throughput",
//	//	io.Magenta + "max rss",
//	//	io.Magenta + "effic",
//	//}
//	//color := io.Grey
//	//if isBest && align {
//	//	color = io.Green
//	//}
//	//indexColor := io.Grey
//	//
//	//if r.ActualExpectRatio < 0.7 && align {
//	//	indexColor = io.Red
//	//} else if r.ActualExpectRatio < 0.85 && align {
//	//	indexColor = io.Yellow
//	//} else {
//	//	indexColor = color
//	//}
//
//	data := []string{
//		color + util.ByteCountDecimal(int64(r.PushedBps)) + "ps",
//		color + fmt.Sprintf("%.1fdps", r.Throughput),
//		// fmt.Sprintf("%s%.1f%%", indexColor, r.ActualExpectRatio*100),
//		io.Grey + r.maxRss(),
//		fmt.Sprintf("%s%s", color, r.efficiency()),
//	}
//
//	if val, ok := combine(inputAttributes, labelsMap)(r)[variable]; ok {
//		header = append(header, io.Magenta+variable)
//		data = append(data, io.Magenta+val)
//	}
//
//	if align {
//		for idx, val := range data {
//			data[idx] = fit(val, len(header[idx]))
//		}
//	}
//
//	// if variable is not a flag, show flags as last column
//	if _, ok := inputAttributes(r)[variable]; ok {
//		header = append(header, io.Magenta+"flags")
//		data = append(data, color+mapToStr(labelsMap(r)))
//	}
//
//	return header, data
//}

// combines all the functions in the argument list into one that returns the same report as calling them in order
func combine(fns ...data) data {
	return func(report TestReport) map[string]string {
		ms := make([]map[string]string, 0)
		for _, fn := range fns {
			ms = append(ms, fn(report))
		}
		return util.Concat(ms...)
	}
}

//// truncates or fills s with spaces so that it has a fixed length (used for visually aligning columns)
//func fit(s string, len int) string {
//	ret := make([]rune, len)
//	var idx int
//	var r rune
//	// surely there should be a simpler implementation...
//	for idx, r = range s {
//		if idx < len {
//			ret[idx] = r
//		} else {
//			break
//		}
//	}
//	for idx2, _ := range ret {
//		if idx2 > idx {
//			ret[idx2] = []rune(" ")[0]
//		}
//	}
//	return string(ret)
//}

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

// returns the slice index of the best performing report, if significant
func best(reports []TestReport) int {
	if len(reports) < 2 {
		return -1
	}
	sorted := make([]TestReport, len(reports))
	copy(sorted, reports)
	sorted = sortBy("throughput", sorted)
	if x := sorted[0].Throughput; x > sorted[1].Throughput*MARGIN && sorted[1].Throughput > 0 {
		for idx, report := range reports {
			if report.Throughput == x {
				return idx
			}
		}
	}
	return -1
}

func randId(seed int64) string {
	rand.Seed(seed)
	l := 8
	runes := []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, l)
	for i := 0; i < l; i++ {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}
