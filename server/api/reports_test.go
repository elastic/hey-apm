package api

import (
	"math"
	"testing"
	"time"

	"fmt"
	"math/rand"
	"strings"

	"github.com/elastic/hey-apm/server/tests"
	"github.com/stretchr/testify/assert"
)

var mb1 int64 = 1000 * 1000

var report = TestReport{
	"01",
	"Wed, 25 Apr 2018 17:17:17 +0200",
	"localhost",
	"rev32341",
	"test_user",
	time.Now().Unix(),
	"python",
	time.Second,
	time.Second,
	1,
	1,
	1,
	1,
	math.MaxInt16,
	1,
	-1,
	"http://localhost:9200",
	"http://localhost:8200",
	"-E apm-server.host=http://localhost:8200 -E output.elasticsearch.hosts=[http://localhost:9200]",
	"master",
	"rev12345678",
	"Fri, 20 Apr 2018 10:00:00 +0200",
	1,
	1,
	mb1,
	1,
}

type builder struct {
	TestReport
}

func newBuilder() *builder {
	r := report
	return &builder{r}
}

func copyBuilder(r TestReport) *builder {
	return &builder{r}
}

func (b *builder) setMaxRss(x int64) *builder {
	b.MaxRss = x
	return b
}

func (b *builder) setReqSize(x int) *builder {
	b.ReqSize = x
	return b
}

func (b *builder) setTotalRes202(x int) *builder {
	b.TotalRes202 = x
	return b
}

func (b *builder) setTotalRes(x int) *builder {
	b.TotalRes = x
	return b
}

func (b *builder) setTotalIndexed(x int64) *builder {
	b.TotalIndexed = x
	return b
}

func (b *builder) setEpr(x int) *builder {
	b.Events = x
	return b
}

func (b *builder) setFpd(x int) *builder {
	b.Frames = x
	return b
}

func (b *builder) setSpt(x int) *builder {
	b.Spans = x
	return b
}

func (b *builder) setConc(x int) *builder {
	b.Concurrency = x
	return b
}

func (b *builder) setDur(d time.Duration) *builder {
	b.Duration = d
	b.Elapsed = d
	return b
}

func (b *builder) setRev(s string) *builder {
	b.Revision = s
	return b
}

func (b *builder) setBranch(s string) *builder {
	b.Branch = s
	return b
}

func (b *builder) setRevDate(s string) *builder {
	b.RevDate = s
	return b
}

func (b *builder) setDate(s string) *builder {
	b.ReportDate = s
	return b
}

func (b *builder) setSize(x int) *builder {
	b.ReqSize = x
	return b
}

func (b *builder) setRes202(x int) *builder {
	b.TotalRes202 = x
	return b
}

func (b *builder) setApm(s string) *builder {
	b.ApmUrl = s
	return b
}

func (b *builder) setEs(s string) *builder {
	b.ElasticUrl = s
	return b
}

func (b *builder) setId(s string) *builder {
	b.ReportId = s
	return b
}

func (b *builder) addFlag(s string) *builder {
	b.ApmFlags += " -E " + s
	return b
}

func TestValidateResult(t *testing.T) {
	for _, test := range []struct {
		tr       TestReport
		unstaged bool
		msg      string
	}{
		{newBuilder().setDur(time.Second * 10).TestReport, false, "duration too short"},
		{newBuilder().TestReport, true, "unstaged changes"},
		{newBuilder().setMaxRss(0).TestReport, false, "memory usage not available"},
		{newBuilder().setBranch("").TestReport, false, "unknown branch"},
		{newBuilder().setRev("").TestReport, false, "unknown revision"},
		{newBuilder().setRevDate("").TestReport, false, "unknown revision date"},
		{newBuilder().setDate("Mon, 01 Jan 0001 00:00:00 +0000").TestReport, false, "work cancelled"},
		{newBuilder().setDur(time.Minute * 1).TestReport, false, "saving report"},
	} {
		msg, b := test.tr.Validate(test.unstaged)
		assert.Contains(t, msg, test.msg)
		if test.msg == "saving report" {
			assert.True(t, b)
		} else {
			assert.False(t, b)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, test := range []struct {
		s1, s2, op string
		b          bool
		e          string
	}{
		{"abc", "abc", "=", true, ""},
		{"abc", "abc", "!=", false, ""},
		{"abc", "abc", ">", false, "comparator > not valid"},
		{"abc", "abc", "<", false, "comparator < not valid"},
		{"abc", "abC", "=", false, ""},
		{"abc", "abC", "!=", true, ""},
		{"1m", "60s", "=", true, ""},
		{"1m", "60s", "!=", false, ""},
		{"1m", "x", "!=", true, "invalid duration"},
		{"1m", "x", "=", false, "invalid duration"},
		{"1m", "1s", "=", false, ""},
		{"1m", "1s", "!=", true, ""},
		{"1m", "1s", ">", true, ""},
		{"1m", "1s", "<", false, ""},
		{"17", "17", "=", true, ""},
		{"17", "17", "!=", false, ""},
		{"17", "18", "<", true, ""},
		{"17", "18", "!=", true, ""},
		{"18", "17", ">", true, ""},
		{"17", "x", "!=", true, "invalid syntax"},
		{"18.7", "18.7", "=", true, ""},
		//don't support floats
		{"18.8", "18.7", ">", false, "comparator > not valid"},
		{"Thu, 26 Apr 2018 15:04:05 -0200", "2018-04-26", "=", true, ""},
		{"Thu, 26 Apr 2018 15:04:05 -0200", "2018-04-26", "!=", false, ""},
		{"Thu, 26 Apr 2018 01:04:05 +0200", "2018-04-26", "=", false, ""},
		{"Thu, 26 Apr 2018 01:04:05 +0200", "2018-04-26", "<", true, ""},
		{"Thu, 26 Apr 2018 23:04:05 -0200", "2018-04-26", ">", true, ""},
		{"Fri, 27 Apr 2018 13:04:05 +0000", "2018-04-26", ">", true, ""},
		{"Fri, 27 Apr 2018 13:04:05 +0000", "2018-04-26", "!=", true, ""},
		{"Wed, 25 Apr 2018 13:04:05 +0000", "2018-04-26", "<", true, ""},
		{"Wed, 25 Apr 2018 13:04:05 +0000", "2018-04-26", "=", false, ""},
		{"2018-04-26", "2018-04-26", "=", true, ""},
		{"2018-04-27", "2018-04-26", ">", false, "comparator > not valid"},
		{"Wed, 25 Apr 2018 13:04:05 +0000", "Fri, 27 Apr 2018 13:04:05 +0000", "<", false, "cannot parse"},
	} {
		ok, err := compare(test.s1, test.s2, test.op)
		assert.Equal(t, test.b, ok)
		if test.e != "" {
			assert.Contains(t, err.Error(), test.e)
		} else {
			assert.NoError(t, err)
		}

	}
}

func TestParseQueryFilters(t *testing.T) {
	ret, err := queryFilters([]string{"a=b", "a$b", "60s!=1m", "x=", "<", "2>1", "", "5 < a"})
	assert.Equal(t, "a$b is not a valid filter, must use one of = != < >", err.Error())
	assert.Len(t, ret, 4)
	assert.Equal(t, ret[0], queryFilter{"a", "b", "="})
	assert.Equal(t, ret[1], queryFilter{"60s", "1m", "!="})
	assert.Equal(t, ret[2], queryFilter{"2", "1", ">"})
	assert.Equal(t, ret[3], queryFilter{"5", "a", "<"})
}

func TestFilterOk(t *testing.T) {
	revDate := "Thu, 26 Apr 2018 17:36:14 +0200"
	reports := []TestReport{
		newBuilder().
			setBranch("branch").setEpr(10).setSpt(5).setConc(1).setRevDate(revDate).
			TestReport,
		newBuilder().
			setEpr(10).setEs("http://127.0.0.1:9200").setSpt(15).setFpd(1).setConc(1).setDur(time.Hour).
			TestReport,
		newBuilder().
			setSize(9999).setEpr(1).setSpt(5).setFpd(1).setConc(1).setRevDate(revDate).
			TestReport,
		newBuilder().
			setSpt(10).setFpd(1).setConc(1).setDur(time.Hour).setRevDate(revDate).
			TestReport,
		newBuilder().
			setBranch("branch").setEpr(10).setConc(0).setDur(time.Hour).setRevDate(revDate).
			TestReport,
		newBuilder().
			setSize(999).setEs("http://localhost:9202").setEpr(10).setSpt(5).setFpd(1).setConc(1).
			TestReport,
		newBuilder().
			setEpr(10).setSpt(5).setFpd(1).setConc(1).setDur(time.Minute).setRevDate(revDate).
			TestReport,
	}

	var err error
	for i, test := range []struct {
		qfs  []queryFilter
		idxs []int
	}{
		{
			[]queryFilter{},
			[]int{0, 1, 2, 3, 4, 5, 6},
		},
		{
			[]queryFilter{
				{"branch", "branch", "="},
			},
			[]int{0, 4},
		},
		{
			[]queryFilter{
				{"spans", "5", "!="},
			},
			[]int{1, 3, 4},
		},
		{
			[]queryFilter{
				{"report_date", "2018-04-25", "!="},
			},
			[]int{},
		},
		{
			[]queryFilter{
				{"concurrency", "0", ">"},
				{"duration", "60m", "="},
			},
			[]int{1, 3},
		},
		{
			[]queryFilter{
				{"concurrency", "0", ">"},
				{"duration", "60m", "="},
			},
			[]int{1, 3},
		},
		{
			[]queryFilter{
				{"request_size", "90", ">"},
				{"request_size", "1000", "<"},
			},
			[]int{5},
		},
		{
			[]queryFilter{
				{"branch", "branch", "!="},
				{"revision_date", "2018-04-23", ">"},
				{"duration", "2m", ">"},
			},
			[]int{3},
		},
		{
			[]queryFilter{
				{"events", "10", "="},
				{"spans", "7", ">"},
			},
			[]int{1},
		},
		{
			// unknown keys are allowed, they could be a valid flag
			[]queryFilter{
				{"x", "10", "="},
			},
			[]int{},
		},
		{
			// "spans > x" is invalid, but because "events = 123" doesn't match, it doesn't get evaluated
			[]queryFilter{
				{"events", "123", "="},
				{"spans", "x", ">"},
			},
			[]int{},
		},
	} {
		var ret map[int]TestReport
		ret, err = filter(query{combine(independentVars, metadata), -1, test.qfs}, reports, err)
		assert.Len(t, ret, len(test.idxs), fmt.Sprintf("found %v at index %d", keys(ret, true), i))
		for _, idx := range test.idxs {
			assert.Equal(t, reports[idx], ret[idx], fmt.Sprintf("failed at index %d %d", i, idx))
		}
	}
	assert.NoError(t, err)
}

func TestFilterFail(t *testing.T) {
	for _, test := range []struct {
		qfs    []queryFilter
		errMsg string
	}{
		{
			[]queryFilter{
				{"events", "1", "="},
				{"spans", "x", ">"},
			},
			"invalid syntax",
		},
		{
			[]queryFilter{
				{"report_date", time.Now().String(), "="},
			},
			"parsing time",
		},
	} {
		query := query{combine(independentVars, metadata), -1, test.qfs}
		_, err := filter(query, []TestReport{newBuilder().TestReport}, nil)
		assert.Contains(t, err.Error(), test.errMsg)
	}
}

func TestSortedAndUnique(t *testing.T) {
	date1 := "Wed, 25 Apr 2018 17:36:14 +0200"
	date2 := "Wed, 25 Apr 2018 17:37:14 +0200"
	date3 := "Thu, 26 Apr 2018 17:00:00 +0200"
	a1 := newBuilder().setId("a1").setBranch("a").setDate(date3).setMaxRss(10).TestReport
	a2 := newBuilder().setId("a2").setBranch("a").setDate(date2).setMaxRss(25).TestReport
	a3 := newBuilder().setId("a3").setBranch("a").setDate(date1).setMaxRss(50).TestReport
	b1 := newBuilder().setId("b1").setBranch("b").setDate(date2).setMaxRss(70).TestReport
	b2 := newBuilder().setId("b2").setBranch("b").setDate(date1).setMaxRss(100).TestReport
	c1 := newBuilder().setId("c1").setBranch("c").setDate(date2).setMaxRss(20).TestReport
	c2 := newBuilder().setId("c2").setBranch("c").setDate(date1).setMaxRss(5).TestReport

	expected := []TestReport{a1, b1, c2}

	for x := 0; x < 1000; x++ {
		rand.Seed(time.Now().UnixNano())
		asIs := []TestReport{a1, a2, a3, b1, b2, c1, c2}
		original := make([]TestReport, 0)
		// randomize order of the input
		for len(asIs) > 0 {
			x := rand.Intn(len(asIs))
			original = append(original, asIs[x])
			asIs = append(asIs[:x], asIs[x+1:]...)
		}
		ret := unique(original)
		assert.Equal(t, expected, ret, original)
	}
}

func TestFindVariants(t *testing.T) {
	template := newBuilder().
		setBranch("branch1").
		setRev("rev1234234").
		setEs("http://localhost:9200").
		setDur(time.Minute).
		setEpr(10).
		TestReport
	a := copyBuilder(template).
		addFlag("mem.queue.size=50").
		TestReport
	b0 := copyBuilder(a).TestReport
	b1 := copyBuilder(a).setBranch("branch2").setRev("rev2122d432").TestReport
	b2 := copyBuilder(a).setRev("rev2122d432").TestReport
	b3 := copyBuilder(a).setEs("http://localhost:9201").TestReport
	b4 := copyBuilder(a).setEs("http://cloud:9200").TestReport
	b5 := copyBuilder(a).setDur(time.Second * 60).TestReport
	b6 := copyBuilder(a).setDur(time.Second).TestReport
	b7 := copyBuilder(a).setEpr(3).TestReport
	b8 := copyBuilder(b7).TestReport
	b9 := copyBuilder(b7).setSpt(5).TestReport
	b10 := copyBuilder(a).setSpt(8).TestReport
	b11 := copyBuilder(a).setFpd(13).TestReport
	b12 := copyBuilder(a).setConc(21).TestReport
	b13 := copyBuilder(template).addFlag("mem.queue.size=70").TestReport
	b14 := copyBuilder(b13).addFlag("apm-server.frontend.enabled=true").TestReport
	b15 := copyBuilder(template).addFlag("apm-server.frontend.enabled=true").TestReport
	b16 := copyBuilder(template).setRev("rev2122d432").addFlag("apm-server.frontend.enabled=true").TestReport

	bs := []TestReport{b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11, b12, b13, b14, b15, b16}
	for idx, test := range []struct {
		attr         string
		expectedIdxs []int
	}{
		{
			"branch", []int{1},
		},
		{
			// b16 not included because is the same revision but has a different flag
			"revision", []int{2},
		},
		{
			"duration", []int{6},
		},
		{
			"events", []int{8, 7},
		},
		{
			"spans", []int{10},
		},
		{
			"frames", []int{11},
		},
		{
			"concurrency", []int{12},
		},
		{
			"mem.queue.size", []int{13},
		},
		// if a flag is not found, findVariants returns reports equivalent to the original
		{
			"apm-server.frontend.enabled", []int{5, 4, 3, 0},
		},
	} {
		ret, err := findVariants(test.attr, a, bs, nil)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedIdxs, keys(ret, true), fmt.Sprintf("at index %d", idx))
	}
}

func TestTop(t *testing.T) {
	now := time.Now()
	aWhileAgo := now.Add(-time.Minute * 10).Format(time.RFC1123Z)
	a := newBuilder().setId("a").TestReport
	b := copyBuilder(a).setId("b").setDate(aWhileAgo).TestReport
	c := copyBuilder(a).setId("c").setRevDate(aWhileAgo).TestReport
	d := copyBuilder(a).setId("d").setTotalRes(30000).TestReport
	e := copyBuilder(a).setId("e").setTotalIndexed(10000).TestReport
	f := copyBuilder(a).setId("f").setRes202(1000).TestReport
	g := copyBuilder(a).setId("g").setMaxRss(10).TestReport
	h := copyBuilder(a).setId("h").setDur(time.Hour * 24).TestReport

	for idx, test := range []struct {
		k, criteria       string
		reports, expected []TestReport
	}{
		{"5", "report_date", []TestReport{a, b}, []TestReport{b, a}},
		{"2", "revision_date", []TestReport{a, c}, []TestReport{c, a}},
		{"2", "pushed_volume", []TestReport{a, d}, []TestReport{d, a}},
		{"1", "index_success_ratio", []TestReport{a, e}, []TestReport{e}},
		{"3", "latency", []TestReport{a, f}, []TestReport{f, a}},
		{"2", "throughput", []TestReport{a, e}, []TestReport{e, a}},
		{"1", "efficiency", []TestReport{a, g}, []TestReport{g}},
		{"1", "duration", []TestReport{a, h}, []TestReport{h}},
		{"0", "efficiency", []TestReport{a, b, c, d}, []TestReport{}},
		{"1s", "efficiency", []TestReport{a, b, c, d}, []TestReport{}},
		{"15m", "efficiency", []TestReport{a, b, c, d}, []TestReport{b}},
	} {
		ret, err := top(test.k, test.criteria, nil, test.reports, nil)
		assert.NoError(t, err)
		ids := make([]string, len(ret))
		for j, r := range ret {
			ids[j] = r.ReportId
		}
		assert.Equal(t, test.expected, ret, fmt.Sprintf("failed at idx %d, got ids %v", idx, ids))
	}
}

func TestCollate(t *testing.T) {
	date1 := "Tue, 01 May 2018 17:00:00 +0200"
	date2 := "Wed, 02 May 2018 17:00:00 +0200"

	template := newBuilder().
		setTotalRes(300).
		setTotalRes202(200).
		setTotalIndexed(6000).
		setSpt(100).
		setFpd(10).
		TestReport
	// will be excluded from query filter
	a := copyBuilder(template).setId("a").TestReport
	// will be excluded from unique because c is the same but more efficient
	b := copyBuilder(template).setId("b").
		setConc(10).setDur(time.Minute * 10).TestReport
	c := copyBuilder(template).setId("c").
		setMaxRss(500000).setDate(date1).TestReport
	// won't be excluded from unique because e and f are different (flag, revision)
	d := copyBuilder(b).setId("d").
		setBranch("branch2").setRev("rev23456").TestReport
	e := copyBuilder(b).setId("e").
		setBranch("branch2").setRev("rev23457").addFlag("flag=1").setMaxRss(5000).TestReport
	f := copyBuilder(b).setId("f").
		setBranch("branch2").setRev("rev23458").setTotalRes202(30).setMaxRss(5000).TestReport
	// won't have variants
	g := copyBuilder(b).setId("g").
		setDur(time.Minute * 20).setDate(date2).TestReport

	reports := []TestReport{a, b, c, d, e, f, g}
	ret, err := collate("10", "branch", "report_date", false, []string{"concurrency>9"}, reports)
	var text = func(idx0, idx1 int) string {
		return tests.WithoutColors(strings.Join(ret[idx0][idx1], " "))
	}
	assert.NoError(t, err)
	assert.Equal(t, "duration 20m0s events 1 spans 100 frames 10 concurrency 10", text(0, 0))
	assert.Equal(t, "report id revision date  pushed    accepted   throughput latency index max rss effic branch flags", text(0, 1))
	assert.Equal(t, "g 18-04-20 10:00 0 bps 0 bps 5.0dps 6000ms 29.7% 1.0Mb 0.000 master ", text(0, 2))
	assert.Equal(t, "duration 10m0s events 1 spans 100 frames 10 concurrency 10", text(1, 0))
	assert.Equal(t, "report id revision date  pushed    accepted   throughput latency index max rss effic branch flags", text(1, 1))
	assert.Equal(t, "b 18-04-20 10:00 0 bps 0 bps 10.0dps 3000ms 29.7% 1.0Mb 0.000 master ", text(1, 2))
	assert.Equal(t, "d 18-04-20 10:00 0 bps 0 bps 10.0dps 3000ms 29.7% 1.0Mb 0.000 branch2 ", text(1, 3))
	assert.Equal(t, "e 18-04-20 10:00 0 bps 0 bps 10.0dps 3000ms 29.7% 5.0kb 0.004 branch2 flag=1 ", text(1, 4))
	assert.Equal(t, "f 18-04-20 10:00 0 bps 0 bps 10.0dps 20000ms 198.0% 5.0kb 0.001 branch2 ", text(1, 5))
}

func TestFit(t *testing.T) {
	assert.Equal(t, "hello the", fit("hello there", 9))
	assert.Equal(t, "hello    ", fit("hello", 9))
	assert.Equal(t, "hello", fit("hello", 5))
	assert.Equal(t, "\x00    ", fit("", 5))
	assert.Equal(t, "", fit("", 0))
}
