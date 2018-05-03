package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"regexp"

	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/tests"
	"github.com/stretchr/testify/assert"
)

func TestInvalidLoadCmds(t *testing.T) {
	for _, invalidCmd := range [][]string{
		{"1", "1", "1", "1", "1"},
		{"1s", "-1", "1", "1", "1"},
		{"1s", "1", "1", "1"},
	} {
		bw := io.NewBufferWriter()
		out, ret := LoadTest(bw, MockState{}, nil, invalidCmd...)
		assert.Contains(t, out, io.Red)
		assert.Equal(t, ret, TestReport{})
	}
}

func TestLoadNotReady(t *testing.T) {
	bw := io.NewBufferWriter()
	cmd := []string{"1s", "1", "0", "1", "1"}
	out, ret := LoadTest(bw, MockState{Ok: errors.New("not ready")}, nil, cmd...)
	assert.Equal(t, "not ready", tests.WithoutColors(out))
	assert.Equal(t, ret, TestReport{})
}

func TestLoadCancelled(t *testing.T) {
	bw := io.NewBufferWriter()
	cmd := []string{"2s", "1", "0", "1", "1"}
	cancel := func() {
		time.Sleep(100 * time.Millisecond)
	}
	s := MockState{MockApm{url: "localhost:822222"}, MockEs{}, nil}
	out, ret := LoadTest(bw, s, cancel, cmd...)
	assert.Equal(t, "\nwork cancelled\n", tests.WithoutColors(out))
	assert.Equal(t, ret, TestReport{})
}

func TestLoadOk(t *testing.T) {
	bw := io.NewBufferWriter()
	cmd := []string{"1s", "1", "2", "1", "0"}
	cancel := func() {
		time.Sleep(time.Second * 2)
	}
	s := MockState{
		MockApm{url: "localhost:822222", branch: "master"},
		MockEs{url: "localhost:922222", docs: 10},
		nil}
	out, ret := LoadTest(bw, s, cancel, cmd...)
	re := regexp.MustCompile("docs indexed (.*)sec")
	out = re.ReplaceAllString(out, "docs indexed (10 / sec")
	assert.Equal(t, `
on branch master , cmd = [1s 1 2 1 0]

pushed 0 b / sec , accepted 0 b / sec
localhost:822222/v1/transactions 0
  total	0 responses (0.00 rps)

10 docs indexed (10 / sec) 
`,
		tests.WithoutColors(out))

	assert.Equal(t, "python", ret.Lang)
	assert.Equal(t, time.Second, ret.Duration)
	assert.Equal(t, 1, ret.Events)
	assert.Equal(t, 2, ret.Spans)
	assert.Equal(t, 1, ret.Frames)
	assert.Equal(t, 3807, ret.ReqSize)
	assert.Equal(t, "localhost:922222", ret.ElasticUrl)
	assert.Equal(t, "localhost:822222", ret.ApmUrl)
	assert.Equal(t, 0, ret.Concurrency)
	assert.Equal(t, 32767, ret.Qps)
	assert.Equal(t, []string(nil), ret.apmFlags())
	assert.Equal(t, "", ret.User)
	assert.Equal(t, "master", ret.Branch)
	assert.Equal(t, 0, ret.TotalRes202)
	assert.Equal(t, float64(0), ret.rps202())
	assert.Equal(t, int64(0), ret.MaxRss)
	assert.Equal(t, int64(10), ret.TotalIndexed)
	assert.Len(t, ret.ReportId, 8)
	assert.InDelta(t, time.Now().Unix(), ret.date().Unix(), time.Second.Seconds())
}

func TestApmTail(t *testing.T) {
	s1 := "an ERROR happened"
	s2 := "we use golang!"
	s3 := "we also use python"
	s4 := "more golang!"
	log := []string{s1, s2, s3, s4}
	for _, test := range []struct {
		withoutColors bool
		n             int
		match         string
		expected      []string
		notExpected   []string
	}{
		{true, 1000, "", log, nil},
		{true, 2, "", []string{s4, s3}, []string{s1, s2}},
		{true, 2, "golang!", []string{s2, s4}, []string{s1, s3}},
		{true, 1, "golang!", []string{s4}, []string{s1, s2, s3}},
		{true, 1, "fortran!", nil, log},
		// yellow included because of the time
		{false, 1, "happened", []string{io.Red, io.Yellow}, nil},
	} {
		tail := Tail(log, test.n, test.match)
		if test.withoutColors {
			tail = tests.WithoutColors(tail)
		}
		for _, exp := range test.expected {
			assert.Contains(t, tail, exp)
		}
		for _, notExp := range test.notExpected {
			assert.NotContains(t, tail, notExp)
		}
	}
}

func TestNameDefinitions(t *testing.T) {
	names := map[string][]string{"a": {"b", "c"}, "1": {"2"}}
	for _, test := range []struct {
		match, expected string
	}{
		// this might be flaky, reports might be read in different order?
		{"", "1 2\na b c\n"},
		{"a", "a b c\n"},
		{"c", "a b c\n"},
		{"3", ""},
	} {
		assert.Equal(t, test.expected, tests.WithoutColors(NameDefinitions(names, test.match)))
	}
	assert.Equal(t, "nothing to show\n", tests.WithoutColors(NameDefinitions(nil, "")))
}

func TestDefine(t *testing.T) {
	reserved := []string{"define"}
	names := map[string][]string{"x": {"1"}}
	for _, test := range []struct {
		cmd            []string
		expectedOut    string
		hasSideEffects bool
		expectedNames  map[string][]string
	}{
		{[]string{"rm", "z"}, "ok", true, names},
		{[]string{"rm", "x"}, "ok", true, map[string][]string{}},
		{[]string{"define", "a"}, "define is a reserved word", false, names},
		{[]string{"a", "1", "2"}, "ok", true, map[string][]string{"x": {"1"}, "a": {"1", "2"}}},
		{[]string{"x", "2"}, "updated old value: 1", true, map[string][]string{"x": {"2"}}},
		{[]string{"a", "1", "a"}, "a can't appear in the right side", false, names},
	} {
		mfw := &tests.MockFileWriter{}
		out, newNames := Define("", mfw, reserved, test.cmd, names)
		assert.Equal(t, test.expectedOut, tests.WithoutColors(out))
		assert.Equal(t, test.expectedNames, newNames)
		assert.Equal(t, test.hasSideEffects, mfw.HasBeenWritenTo())
	}
}

func TestDump(t *testing.T) {
	mfw := &tests.MockFileWriter{}
	out := Dump(mfw, "json", "1", "0", "1")
	assert.Equal(t, "2.5kb written to disk\n", tests.WithoutColors(out))
	assert.Equal(t, "\n{\"errors\":[\n{\"context\":\n\t{\"custom\":{},\n\t\"request\":{\"body\":null,\n\t\t\t\t\"cookies\":{},\n\t\t\t\t\"env\":{\"REMOTE_ADDR\":\"127.0.0.1\",\"SERVER_NAME\":\"1.0.0.127.in-addr.arpa\",\"SERVER_PORT\":\"8000\"},\n\t\t\t\t\"headers\":{\"accept\":\"*/*\",\"accept-encoding\":\"gzip, deflate, br\",\"accept-language\":\"en-US,en;q=0.9\",\"connection\":\"keep-alive\",\"content-length\":\"\",\"content-type\":\"text/plain\",\"host\":\"localhost:8000\",\"referer\":\"http://localhost:8000/dashboard\",\"user-agent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36\"},\n\t\t\t\t\"method\":\"GET\",\n\t\t\t\t\"socket\":{\"encrypted\":false,\"remote_address\":\"127.0.0.1\"},\n\t\t\t\t\"url\":{\"full\":\"http://localhost:8000/api/stats\",\"hostname\":\"localhost\",\"pathname\":\"/api/stats\",\"port\":\"8000\",\"protocol\":\"http:\"}\n\t\t\t\t},\n\t\"user\":{\"id\":null,\"is_authenticated\":false,\"username\":\"\"}\n\t},\n\"culprit\":\"opbeans.views.stats\",\n\"exception\":\n\t{\"message\":\"ConnectionError: Error 61 connecting to localhost:6379. Connection refused.\",\n\t\t\"module\":\"redis.exceptions\",\n\t\t\"stacktrace\":[\n{\"abs_path\":\"/opbeans/lib/python3.6/site-packages/redis/client.py\",\"context_line\":\"            connection.send_command(*args)\",\"filename\":\"redis/client.py\",\"function\":\"execute_command\",\"library_frame\":true,\"lineno\":673,\"module\":\"redis.client\",\"post_context\":[\"            return self.parse_response(connection, command_name, **options)\",\"        finally:\"],\"pre_context\":[\"            connection.disconnect()\",\"            if not connection.retry_on_timeout and isinstance(e, TimeoutError):\",\"                raise\"],\"vars\":{\"args\":[\"GET\",\"cache:1:shop-stats\"],\"command_name\":\"GET\",\"connection\":\"Connection\\u003chost=localhost,port=6379,db=1\\u003e\",\"options\":{},\"pool\":\"ConnectionPool\\u003cConnection\\u003chost=localhost,port=6379,db=1\\u003e\\u003e\",\"self\":\"StrictRedis\\u003cConnectionPool\\u003cConnection\\u003chost=localhost,port=6379,db=1\\u003e\\u003e\\u003e\"}}\n],\n \t\t\"type\":\"ConnectionError\",\n\t\t\"handled\":false\n\t},\n\n\"id\":\"e99fd5d7-516f-422d-a6fe-3550a49283e0\",\n\"timestamp\":\"2018-01-09T03:35:37.604813Z\",\n\"transaction\":{\"id\":\"87d45146-e0ce-4a04-877c-a672921df059\"}}\n],\n\"process\":{\"argv\":[\"./manage.py\",\"runserver\"],\"pid\":52687,\"title\":null},\n\"service\":{\"agent\":{\"name\":\"python\",\"version\":\"1.0.0\"},\n\t\t\t\"environment\":null,\n\t\t\t\"framework\":{\"name\":\"django\",\"version\":\"1.11.8\"},\n\t\t\t\"language\":{\"name\":\"python\",\"version\":\"3.6.3\"},\n\t\t\t\"name\":\"opbeans-python\",\n\t\t\t\"runtime\":{\"name\":\"CPython\",\"version\":\"3.6.3\"},\"version\":null},\n\"system\":{\"architecture\":\"x86_64\",\"hostname\":\"localhost.localdomain\",\"platform\":\"darwin\"}\n}\n",
		mfw.Data)

	out = Dump(mfw, "json", "1", "1", "1")
	assert.Equal(t, "2.8kb written to disk\n", tests.WithoutColors(out))
	assert.Equal(t, "\n{\"transactions\":[\n{\"spans\":[\n{\"stacktrace\":[\n{\"abs_path\":\"/opbeans/lib/python3.6/site-packages/redis/client.py\",\"context_line\":\"            connection.send_command(*args)\",\"filename\":\"redis/client.py\",\"function\":\"execute_command\",\"library_frame\":true,\"lineno\":673,\"module\":\"redis.client\",\"post_context\":[\"            return self.parse_response(connection, command_name, **options)\",\"        finally:\"],\"pre_context\":[\"            connection.disconnect()\",\"            if not connection.retry_on_timeout and isinstance(e, TimeoutError):\",\"                raise\"],\"vars\":{\"args\":[\"GET\",\"cache:1:shop-stats\"],\"command_name\":\"GET\",\"connection\":\"Connection\\u003chost=localhost,port=6379,db=1\\u003e\",\"options\":{},\"pool\":\"ConnectionPool\\u003cConnection\\u003chost=localhost,port=6379,db=1\\u003e\\u003e\",\"self\":\"StrictRedis\\u003cConnectionPool\\u003cConnection\\u003chost=localhost,port=6379,db=1\\u003e\\u003e\\u003e\"}}\n],\n\"context\":null,\n\"duration\":0.12803077697753906,\n\"id\":0,\n\"start\":1.4657974243164062,\n\"type\":\"template.django\",\n\"name\":\"\\u003ctemplate string\\u003e\",\n\"parent\":null\n}\n],\n\"context\":\n\t{\"request\":\n\t\t{\"body\":null,\n\t\t\"cookies\":{},\n\t\t\"env\":{\"REMOTE_ADDR\":\"127.0.0.1\",\"SERVER_NAME\":\"1.0.0.127.in-addr.arpa\",\"SERVER_PORT\":\"8000\"},\n\t\t\"headers\":{\"accept\":\"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8\",\"accept-encoding\":\"gzip, deflate, br\",\"accept-language\":\"en-US,en;q=0.9\",\"connection\":\"keep-alive\",\"content-length\":\"\",\"content-type\":\"text/plain\",\"host\":\"localhost:8000\",\"upgrade-insecure-requests\":\"1\",\"user-agent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36\"},\n\t\t\"method\":\"GET\",\n\t\t\"socket\":{\"encrypted\":false,\"remote_address\":\"127.0.0.1\"},\n\t\t\"url\":{\"full\":\"http://localhost:8000/\",\"hostname\":\"localhost\",\"pathname\":\"/\",\"port\":\"8000\",\"protocol\":\"http:\"}\n\t},\n\t\"response\":{\"headers\":{\"content-length\":\"365\",\"content-type\":\"text/html; charset=utf-8\",\"x-frame-options\":\"SAMEORIGIN\"},\n\t\t\t\t\"status_code\":200},\n\t\"tags\":{},\n\t\"user\":{\"id\":null,\"is_authenticated\":false,\"username\":\"\"}\n},\n\"timestamp\":\"2018-01-09T03:35:37.604813Z\",\n\"type\":\"request\",\n\"duration\":25.555133819580078,\n\"id\":\"9eb1899c-f767-4f40-85af-e2de18aaaf0c\",\n\"name\":\"GET django.views.generic.base.TemplateView\",\n\"result\":\"HTTP 2xx\",\n\"sampled\":true\n}\n],\n\"process\":{\"argv\":[\"./manage.py\",\"runserver\"],\"pid\":52687,\"title\":null},\n\"service\":{\"agent\":{\"name\":\"python\",\"version\":\"1.0.0\"},\n\t\"environment\":null,\n\t\"framework\":{\"name\":\"django\",\"version\":\"1.11.8\"},\n\t\"language\":{\"name\":\"python\",\"version\":\"3.6.3\"},\n\t\"name\":\"opbeans-python\",\n\t\"runtime\":{\"name\":\"CPython\",\"version\":\"3.6.3\"},\"version\":null},\n\"system\":{\"architecture\":\"x86_64\",\"hostname\":\"localhost.localdomain\",\"platform\":\"darwin\"}\n}\n",
		mfw.Data)

	out = Dump(mfw, "json", "a")
	assert.Contains(t, out, "invalid syntax")
}

func TestStatus(t *testing.T) {
	state := MockState{
		MockApm{dir: "NOTADIR", url: "localhost:8200"},
		MockEs{url: "localhost:9200", health: "great", docs: 17},
		nil,
	}
	out := Status(state).String()
	assert.Equal(t,
		`ElasticSearch [localhost:9200]: great ,  17 docs
ApmServer [localhost:8200]: not running
Can't ch to directory NOTADIR (hint: apm use <dir>)
`,
		tests.WithoutColors(out))
}

func TestStatusExternal(t *testing.T) {
	if os.Getenv("SKIP_EXTERNAL") == "" {
		return
	}
	heyDir := filepath.Join(os.Getenv("GOPATH"), "src/github.com/elastic/hey-apm")
	state := MockState{
		MockApm{dir: heyDir, url: "localhost:8200", branch: "master", running: true},
		MockEs{url: "localhost:9200", HealthErr: errors.New("broken"), docs: 0},
		nil,
	}
	out := Status(state).String()
	ok := true
	for _, keyword := range []string{"broken", "running", "Using", "master"} {
		ok = ok && assert.Contains(t, out, keyword)
	}

	if !ok {
		fmt.Println("\nthis test have external dependencies: hey-apm in the expected directory")
		fmt.Println("you might disable external tests with SKIP_EXTERNAL=1 go test -v ./...")
		fmt.Println()
	}
}
