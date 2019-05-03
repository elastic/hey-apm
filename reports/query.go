package reports

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	s "strings"
	"time"

	"github.com/elastic/hey-apm/out"

	"github.com/elastic/hey-apm/strcoll"
)

// filters and sorts `reports` and for each result and returns a digest matrix
// each row is the digest of a report with all user-entered attributes equal but one
// for more details check out the Readme and the `reports.collate` function
// TODO add validation and return error
func Collate(size int, since time.Duration, sort string, args []string, reports []Report) string {
	variable := strcoll.Get(0, args)
	bw := out.NewBufferWriter()
	digests, err := collate(since, size, variable, sort, strcoll.From(1, args), reports)
	if err != nil {
		out.ReplyEitherNL(bw, err)
	} else {
		for _, group := range digests {
			for _, line := range group {
				out.ReplyNL(bw, s.Join(line, "\t"))
			}
			out.Reply(bw, "\n")
		}
		if len(digests) == 0 {
			out.Reply(bw, "\n")
		}
	}
	return bw.String()
}

// functions of this type map a subset of attribute names to their (stringified) values
// queries are performed against such maps
type data func(Report) map[string]string

// returns a map of attributes provided by the user, excluding labels
func inputAttributes(r Report) map[string]string {
	return map[string]string{
		// TODO
		"duration":    fmt.Sprintf("%.2f", r.RunTimeout),
		"apm_host":    r.ApmHost,
		"apms":        strconv.Itoa(r.NumApms),
		"apm_version": r.ApmVersion,
		"es_host":     r.ElasticHost,
	}
}

// attributes that makes sense to filter by, but not breakdown by
func metadata(r Report) map[string]string {
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
	// intKeys in query filters are expected to be a subset of intKeys in data
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
// `filtersExpr` must include all the independent variables except apm_host
// `all` must not be empty
// TODO needs build_date
func verify(since time.Duration, filtersExpr []string, all []Report) (bool, error) {
	if len(all) == 0 {
		return false, errors.New("no reports")
	}
	filters, err := queryFilters(filtersExpr)
	filterKeys := make([]string, 0)
	for _, filter := range filters {
		filterKeys = append(filterKeys, filter.k)
	}
	for k, _ := range inputAttributes(all[0]) {
		// exclude "apm_host"?
		if !strcoll.Contains(k, filterKeys) {
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

// applies the given filters to `all`, and returns the top reports sorted by `sortCriteria`
// then, for each report it finds other reports with the same test parameters values except for `variable`
func collate(since time.Duration, size int, variable, sortCriteria string, filtersExpr []string, all []Report) ([][][]string, error) {
	ret := make([][][]string, 0)
	// keep track of observed reports to avoid duplicated in results
	ids := make([]string, 0)
	var newReports []Report
	filters, err := queryFilters(filtersExpr)
	all = unique(all)
	reports, err := top(since, size, sortCriteria, filters, all, err)
	for _, report := range reports {
		var variants []Report
		variants, err = values(findVariants(variable, report, all, err))
		variants, ids = seen(variants, ids)
		newReports, ids = seen([]Report{report}, ids)
		if len(newReports) == 0 {
			continue
		}
		// maybe a different sorting criteria would be better?
		variants, err = top(since, size, sortCriteria, nil, unique(variants), err)
		// best := best(append(newReports, variants...))
		digestMatrix := make([][]string, len(variants)+3)
		digestMatrix[0], digestMatrix[1] = digest(report, variable)
		for idx, variant := range variants {
			_, digestMatrix[3+idx] = digest(variant, variable)
		}
		ret = append(ret, digestMatrix)
	}
	return ret, err
}

func top(since time.Duration, size int, criteria string, filters []queryFilter, reports []Report, err error) ([]Report, error) {
	reports = sortBy(criteria, reports)
	// TODO add labelsMap
	query := query{combine(inputAttributes, metadata), -1, filters}
	ret, err := values(filter(query, reports, err))
	return head(since, size, ret), err
}

func labelsMap(r Report) map[string]string {
	ret := make(map[string]string)
	for _, label := range r.Labels {
		split := s.Split(label, "=")
		ret[s.TrimSpace(split[0])] = s.TrimSpace(split[1])
	}
	return ret
}

// returns a subset of `bs` with the same inputAttributes values as `a` except for `variable`, which must be different
// returned reports are keyed by their index in the original `bs` slice
func findVariants(attribute string, a Report, bs []Report, err error) (map[int]Report, error) {
	filters := make([]queryFilter, 0)
	var data data
	if _, ok := inputAttributes(a)[attribute]; ok {
		data = inputAttributes
	} else {
		// consider apm server labels only when comparing the same revision or a unknown attribute (eg flag)
		// in different revisions labels might be different / non comparable
		data = combine(inputAttributes, labelsMap)
	}
	for k, v := range data(a) {
		// special case: 2 results with different number of apms can't have the same apm_host value
		if k == attribute || (attribute == "apms" && k == "apm_host") {
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
func filter(query query, reports []Report, err error) (map[int]Report, error) {
	ret := make(map[int]Report)
OuterLoop:
	for idx, report := range reports {
		data := query.data(report)
		// if query.size != -1 && query.size != len(data) {
		// this happens when comparing reports with different (number of) labels
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
	} else if t1, err1 := time.Parse(GITRFC, s1); err1 == nil {
		t2, err2 := time.Parse(HUMAN, s2)
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

func sortBy(criteria string, reports []Report) []Report {
	switch criteria {
	case "report_date":
		sort.Sort(descByReportDate{reports})
	// case "build_date":
	//	sort.Sort(descByRevDate{reports})
	case "duration":
		sort.Sort(descByDuration{reports})
	}
	return reports
}

// if 2 reports have the same independent variables, return the one that showed better performance
// reports are sorted by their date, most recent first
func unique(reports []Report) []Report {
	return uniq(sortBy("report_date", reports))
}

func uniq(reports []Report) []Report {
	uniques := make([]Report, 0)
	if len(reports) == 0 {
		return uniques
	}
	first, rest := reports[0], reports[1:]
	variant, _ := findVariants("", first, rest, nil)
	isUnique := true
	for _, k := range intKeys(variant, true) {
		if *first.EventIndexRate > *variant[k].EventIndexRate {
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

// return the first `size` reports within the given time frame
func head(since time.Duration, size int, reports []Report) []Report {
	ret := make([]Report, 0)
	for _, report := range reports {
		if report.date().Add(since).After(time.Now()) {
			ret = append(ret, report)
		}
	}
	return ret[:int(math.Min(float64(size), float64(len(ret))))]
}

// returns the values of the given map in order
func values(m map[int]Report, err error) ([]Report, error) {
	ret := make([]Report, len(m))
	for idx, k := range intKeys(m, false) {
		ret[idx] = m[k]
	}
	return ret, err
}

// returns the intKeys of the given map in ascending or descending order
func intKeys(m map[int]Report, desc bool) []int {
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
func seen(reports []Report, ids []string) ([]Report, []string) {
	ret := make([]Report, 0)
	for _, report := range reports {
		if !strcoll.Contains(report.ReportId, ids) {
			ids = append(ids, report.ReportId)
			ret = append(ret, report)
		}
	}
	return ret, ids
}

//// a digest describes the most informative performance data
func digest(r Report, variable string) ([]string, []string) {
	header := make([]string, 0)
	data := make([]string, 0)
	if val, ok := combine(inputAttributes, labelsMap)(r)[variable]; ok {
		header = append(header, variable)
		data = append(data, val)
	}
	for k, v := range inputAttributes(r) {
		if k != variable {
			header = append(header, k)
			data = append(data, v)
		}
	}
	header = append(header, "throughput")
	header = append(header, "pushed")
	data = append(data, fmt.Sprintf("%.1fdps", r.EventIndexRate))
	return header, data
}

// combines all the functions in the argument list into one that returns the same report as calling them in order
func combine(fns ...data) data {
	return func(report Report) map[string]string {
		ms := make([]map[string]string, 0)
		for _, fn := range fns {
			ms = append(ms, fn(report))
		}
		return strcoll.Concat(ms...)
	}
}

const MARGIN = 1.33

// returns the slice index of the best performing report, if significant
func best(reports []Report) int {
	if len(reports) < 2 {
		return -1
	}
	sorted := make([]Report, len(reports))
	copy(sorted, reports)
	sorted = sortBy("throughput", sorted)
	if x := *sorted[0].EventIndexRate; x > *sorted[1].EventIndexRate*MARGIN && *sorted[1].EventIndexRate > 0 {
		for idx, report := range reports {
			if *report.EventIndexRate == x {
				return idx
			}
		}
	}
	return -1
}
