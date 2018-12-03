package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/tests"
	"github.com/elastic/hey-apm/target"
	"github.com/stretchr/testify/assert"
)

func basicTarget(t *testing.T, opts ...target.OptionFunc) *target.Target {
	required := []target.OptionFunc{
		target.RunTimeout("1s"),
		target.NumTransactions("1"),
		target.NumSpans("1"),
		target.NumFrames("1"),
		// set number of agents to 0 so the worker doesn't actually run
		target.NumAgents("0"),
		target.Throttle("1"),
	}
	target, err := target.NewTargetFromOptions([]string{""}, append(required, opts...)...)
	assert.NoError(t, err)
	return target
}

func TestLoadNotReady(t *testing.T) {
	bw := io.NewBufferWriter()
	ret := LoadTest(bw, MockState{Ok: errors.New("not ready")}, nil, time.Duration(0), *basicTarget(t))
	assert.Equal(t, "not ready", tests.WithoutColors(bw.String()))
	assert.Equal(t, ret, TestResult{Cancelled: true})
}

func TestLoadCancelled(t *testing.T) {
	bw := io.NewBufferWriter()
	cancel := func() {
		time.Sleep(100 * time.Millisecond)
	}
	s := MockState{MockApm{url: "localhost:822222"}, MockEs{}, nil}
	ret := LoadTest(bw, s, cancel, time.Duration(0), *basicTarget(t))
	assert.Equal(t, ret, TestResult{Cancelled: true})
}

func TestLoadOk(t *testing.T) {
	bw := io.NewBufferWriter()
	cancel := func() {
		time.Sleep(time.Second * 2)
	}
	s := MockState{
		MockApm{url: "localhost:822222", branch: "master"},
		MockEs{url: "localhost:922222", docs: 10},
		nil}
	ret := LoadTest(bw, s, cancel, time.Duration(0), *basicTarget(t))
	assert.Contains(t, bw.String(), "started new work")

	assert.Equal(t, time.Second, ret.Duration)
	assert.Equal(t, 0, ret.Errors)
	assert.Equal(t, 1, ret.Transactions)
	assert.Equal(t, 1, ret.Spans)
	assert.Equal(t, 1, ret.Frames)
	assert.Equal(t, "localhost:922222", ret.ElasticUrl)
	assert.Equal(t, "localhost:822222", ret.ApmUrls)
	assert.Equal(t, 0, ret.Agents)
	assert.Equal(t, 1, ret.Throttle)
	assert.Equal(t, "master", ret.Branch)
	// assert.Equal(t, 0, ret.AcceptedResponses)
	// assert.Equal(t, float64(0), ret.AcceptedRps)
	assert.Equal(t, int64(0), ret.ActualDocs)
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
	out := Dump(mfw, "json", "1", "1", "1", "1")
	assert.Contains(t, out, "written to disk", tests.WithoutColors(out))
	for _, jsonKey := range []string{"\"metadata\"", "\"user\"", "\"process\"", "\"system\"",
		"\"service\"", "\"transaction\"", "\"error\"", "\"span\"", "\"abs_path\""} {
		assert.Contains(t, mfw.Data, jsonKey)
	}

	out = Dump(mfw, "json", "0", "1", "1", "1")
	assert.NotContains(t, mfw.Data, "\"error\"")

	out = Dump(mfw, "json", "1", "0", "1", "1")
	assert.NotContains(t, mfw.Data, "\"transaction\"")

	out = Dump(mfw, "json", "1", "1", "0", "1")
	assert.NotContains(t, mfw.Data, "\"span\"")

	out = Dump(mfw, "json", "1", "1", "1", "0")
	assert.NotContains(t, mfw.Data, "\"abs_path\"")

	out = Dump(mfw, "json", "a")
	assert.Contains(t, out, "invalid syntax")
}

func TestStatus(t *testing.T) {
	state := MockState{
		MockApm{dir: "dir", url: "localhost:8200"},
		MockEs{url: "localhost:9200", health: "great", docs: 17},
		nil,
	}
	out := Status(state).String()
	assert.Equal(t,
		`ElasticSearch [localhost:9200]: great ,  17 docs
ApmServer [localhost:8200]: not running

Using dir
Git Info: unknown branch (hint: apm switch <branch>)
`,
		tests.WithoutColors(out))
}

func TestStatusExternal(t *testing.T) {
	if os.Getenv("SKIP_EXTERNAL") != "" {
		fmt.Println("skipping status test")
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
