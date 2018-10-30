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
	"github.com/stretchr/testify/assert"
	"github.com/elastic/hey-apm/target"
)

func basicTarget(t *testing.T, opts ...target.OptionFunc) *target.Target {
	required := []target.OptionFunc {
		target.RunTimeout("1s"),
		target.NumTransactions("1"),
		target.NumSpans("1"),
		target.NumFrames("1"),
		// set number of agents to 0 so the worker doesn't actually run
		target.NumAgents("0"),
		target.Throttle("1"),
	}
	target, err := target.NewTargetFromOptions("", append(required, opts...)...)
	assert.NoError(t, err)
	return target
}

func TestLoadNotReady(t *testing.T) {
	bw := io.NewBufferWriter()
	ret := LoadTest(bw, MockState{Ok: errors.New("not ready")}, nil, *basicTarget(t))
	assert.Equal(t, "not ready", tests.WithoutColors(bw.String()))
	assert.Equal(t, ret, TestResult{Cancelled: true})
}

func TestLoadCancelled(t *testing.T) {
	bw := io.NewBufferWriter()
	cancel := func() {
		time.Sleep(100 * time.Millisecond)
	}
	s := MockState{MockApm{url: "localhost:822222"}, MockEs{}, nil}
	ret := LoadTest(bw, s, cancel, *basicTarget(t))
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
	ret := LoadTest(bw, s, cancel, *basicTarget(t))
	assert.Equal(t, `started new work, url /intake/v2/events, payload size 2.7kb (uncompressed), 1.3kb (compressed) ...
>>> 
sent 2 events per request with 0 agent(s) throttled at 1 requests per second

total 0 responses (0.00 rps)
`,
		tests.WithoutColors(bw.String()))

	assert.Equal(t, time.Second, ret.Duration)
	assert.Equal(t, 0, ret.Errors)
	assert.Equal(t, 1, ret.Transactions)
	assert.Equal(t, 1, ret.Spans)
	assert.Equal(t, 1, ret.Frames)
	assert.Equal(t, 1309, ret.GzipReqSize)
	assert.Equal(t, "localhost:922222", ret.ElasticUrl)
	assert.Equal(t, "localhost:822222", ret.ApmUrl)
	assert.Equal(t, 0, ret.Agents)
	assert.Equal(t, 1, ret.Throttle)
	assert.Equal(t, "master", ret.Branch)
	assert.Equal(t, 0, ret.AcceptedResponses)
	assert.Equal(t, float64(0), ret.AcceptedRps)
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
	assert.Equal(t, "4.8kb written to disk\n", tests.WithoutColors(out))
	expected := `{"metadata": {"user": {"id": "123", "email": "s@test.com", "username": "john"}, "process": {"ppid": 6789, "pid": 1234,"argv": ["node", "server.js"], "title": "node"}, "system": {"platform": "darwin", "hostname": "prod1.example.com", "architecture": "x64"}, "service":{"name": "backendspans", "language": {"version": "8", "name": "ecmascript"}, "agent": {"version": "3.14.0", "name": "elastic-node"}, "environment": "staging", "framework": {"version": "1.2.3", "name": "Express"}, "version": "5.1.3", "runtime": {"version": "8.0.0", "name": "node"}}}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"result": "success",  "sampled": true, "span_count": {"started": 17},"context":{"request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response":{"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true}, "user": {"id": "99","username": "foo","email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"error": {"id": "0123456789012345", "culprit": "my.module.function_name","log":{"message": "My service could not talk to the database named foobar", "param_message": "My service could not talk to the database named %s", "logger_name": "my.logger.name", "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "level": "warning"},"exception":{"message": "The username root is unknown","type": "DbError","module": "__builtins__","code": 42,"attributes": {"foo": "bar" },"stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "handled": false},"context":{"request":{"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url":{"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string", "hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]}, "cookies": {"c1": "v1", "c2": "v2" },"env": {"SERVER_SOFTWARE": "nginx", "GATEWAY_INTERFACE": "CGI/1.1"},"body": "Hello World"},"response":{"status_code": 200, "headers": {"content-type": "application/json"},"headers_sent": true, "finished": true}, "user": {"id": 99, "username": "foo", "email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}`
	assert.Equal(t, expected, mfw.Data)

	out = Dump(mfw, "json", "3", "3", "3", "3")
	expected = `{"metadata": {"user": {"id": "123", "email": "s@test.com", "username": "john"}, "process": {"ppid": 6789, "pid": 1234,"argv": ["node", "server.js"], "title": "node"}, "system": {"platform": "darwin", "hostname": "prod1.example.com", "architecture": "x64"}, "service":{"name": "backendspans", "language": {"version": "8", "name": "ecmascript"}, "agent": {"version": "3.14.0", "name": "elastic-node"}, "environment": "staging", "framework": {"version": "1.2.3", "name": "Express"}, "version": "5.1.3", "runtime": {"version": "8.0.0", "name": "node"}}}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"result": "success",  "sampled": true, "span_count": {"started": 17},"context":{"request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response":{"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true}, "user": {"id": "99","username": "foo","email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"result": "success",  "sampled": true, "span_count": {"started": 17},"context":{"request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response":{"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true}, "user": {"id": "99","username": "foo","email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"transaction": {"id": "4340a8e0df1906ecbfa9", "trace_id": "0acd456789abcdef0123456789abcdef", "name": "GET /api/types","type": "request","duration": 32.592981,"result": "success",  "sampled": true, "span_count": {"started": 17},"context":{"request": {"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url": {"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string","hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},"cookies": {"c1": "v1","c2": "v2"},"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},"response":{"status_code": 200,"headers": {"content-type": "application/json"},"headers_sent": true,"finished": true}, "user": {"id": "99","username": "foo","email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"span": {"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "context":{"db":{"instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": {"url": "http://localhost:8000"}}}}
{"error": {"id": "0123456789012345", "culprit": "my.module.function_name","log":{"message": "My service could not talk to the database named foobar", "param_message": "My service could not talk to the database named %s", "logger_name": "my.logger.name", "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "level": "warning"},"exception":{"message": "The username root is unknown","type": "DbError","module": "__builtins__","code": 42,"attributes": {"foo": "bar" },"stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "handled": false},"context":{"request":{"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url":{"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string", "hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]}, "cookies": {"c1": "v1", "c2": "v2" },"env": {"SERVER_SOFTWARE": "nginx", "GATEWAY_INTERFACE": "CGI/1.1"},"body": "Hello World"},"response":{"status_code": 200, "headers": {"content-type": "application/json"},"headers_sent": true, "finished": true}, "user": {"id": 99, "username": "foo", "email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"error": {"id": "0123456789012345", "culprit": "my.module.function_name","log":{"message": "My service could not talk to the database named foobar", "param_message": "My service could not talk to the database named %s", "logger_name": "my.logger.name", "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "level": "warning"},"exception":{"message": "The username root is unknown","type": "DbError","module": "__builtins__","code": 42,"attributes": {"foo": "bar" },"stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "handled": false},"context":{"request":{"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url":{"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string", "hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]}, "cookies": {"c1": "v1", "c2": "v2" },"env": {"SERVER_SOFTWARE": "nginx", "GATEWAY_INTERFACE": "CGI/1.1"},"body": "Hello World"},"response":{"status_code": 200, "headers": {"content-type": "application/json"},"headers_sent": true, "finished": true}, "user": {"id": 99, "username": "foo", "email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}
{"error": {"id": "0123456789012345", "culprit": "my.module.function_name","log":{"message": "My service could not talk to the database named foobar", "param_message": "My service could not talk to the database named %s", "logger_name": "my.logger.name", "stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "level": "warning"},"exception":{"message": "The username root is unknown","type": "DbError","module": "__builtins__","code": 42,"attributes": {"foo": "bar" },"stacktrace":[{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]},{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}], "handled": false},"context":{"request":{"socket": {"remote_address": "12.53.12.1","encrypted": true},"http_version": "1.1","method": "POST","url":{"protocol": "https:","full": "https://www.example.com/p/a/t/h?query=string#hash","hostname": "www.example.com","port": "8080","pathname": "/p/a/t/h","search": "?query=string", "hash": "#hash","raw": "/p/a/t/h?query=string#hash"},"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]}, "cookies": {"c1": "v1", "c2": "v2" },"env": {"SERVER_SOFTWARE": "nginx", "GATEWAY_INTERFACE": "CGI/1.1"},"body": "Hello World"},"response":{"status_code": 200, "headers": {"content-type": "application/json"},"headers_sent": true, "finished": true}, "user": {"id": 99, "username": "foo", "email": "foo@example.com"},"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]}}}}}`
	assert.Equal(t, expected, mfw.Data)

	out = Dump(mfw, "json", "0", "0", "1", "0")
	expected = `{"metadata": {"user": {"id": "123", "email": "s@test.com", "username": "john"}, "process": {"ppid": 6789, "pid": 1234,"argv": ["node", "server.js"], "title": "node"}, "system": {"platform": "darwin", "hostname": "prod1.example.com", "architecture": "x64"}, "service":{"name": "backendspans", "language": {"version": "8", "name": "ecmascript"}, "agent": {"version": "3.14.0", "name": "elastic-node"}, "environment": "staging", "framework": {"version": "1.2.3", "name": "Express"}, "version": "5.1.3", "runtime": {"version": "8.0.0", "name": "node"}}}}`
	assert.Equal(t, expected, mfw.Data)


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
