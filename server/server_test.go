package server

import (
	"strings"
	"testing"
	"time"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/client"
	"github.com/elastic/hey-apm/server/tests"
	"github.com/stretchr/testify/assert"
)

func TestPeekAndFilter(t *testing.T) {
	bw := io.NewBufferWriter()
	conn := client.WrapConnection(nil)
	for _, cmd := range []string{"a", "b", "c", "b"} {
		conn.EvalCh <- []string{cmd}
	}
	peekAndFilter(bw, "", conn)
	assert.Equal(t,
		`4 commands queued
 86f7e4  [a]
 e9d71f  [b]
 84a516  [c]
 e9d71f  [b]
`,
		tests.WithoutColors(bw.String()))

	bw = io.NewBufferWriter()
	peekAndFilter(bw, "e9d71f", conn)
	assert.Equal(t,
		`2 commands queued
 86f7e4  [a]
 84a516  [c]
`,
		tests.WithoutColors(bw.String()))
}

func TestEvalPush(t *testing.T) {
	conn := client.WrapConnection(nil)
	go func() {
		<-conn.EvalCh
	}()
	ret := eval([]string{"define", "x", "1"}, conn, api.MockState{})
	assert.Equal(t, "", tests.WithoutColors(ret))
	assert.Len(t, conn.EvalCh, 0)

	ret = eval([]string{"cmd"}, conn, api.MockState{})
	assert.Equal(t, "(queued)\n", tests.WithoutColors(ret))
	assert.Len(t, conn.EvalCh, 1)
}

func TestEvalCancel(t *testing.T) {
	conn := client.WrapConnection(nil)
	var cancelled bool
	go func() {
		conn.CancelSig.L.Lock()
		defer conn.CancelSig.L.Unlock()
		conn.CancelSig.Wait()
		cancelled = true
	}()
	eval([]string{"a"}, conn, api.MockState{})
	ret := eval([]string{"cancel"}, conn, api.MockState{})

	assert.Equal(t, "ok\n", tests.WithoutColors(ret))
	assert.Len(t, conn.EvalCh, 1)
	time.Sleep(200 * time.Millisecond)
	assert.True(t, cancelled)
}

func TestEvalCancelCmd(t *testing.T) {
	conn := client.WrapConnection(nil)
	var cancelled bool
	go func() {
		conn.CancelSig.L.Lock()
		defer conn.CancelSig.L.Unlock()
		conn.CancelSig.Wait()
		cancelled = true
	}()
	eval([]string{"a"}, conn, api.MockState{})
	ret := eval([]string{"cancel", "86f7e4"}, conn, api.MockState{})

	assert.Equal(t, "0 commands queued\n", tests.WithoutColors(ret))
	assert.Len(t, conn.EvalCh, 0)
	assert.False(t, cancelled)
}

func TestEvalTail(t *testing.T) {
	conn := client.WrapConnection(nil)
	l := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n"}
	state := api.MockState{MockApm: api.MockApm{L: l}}
	for _, test := range []struct {
		cmd           []string
		expectedLines int
	}{
		{[]string{"apm", "tail"}, 12},
		{[]string{"apm", "tail", "-100"}, 16},
		{[]string{"apm", "tail", "-1"}, 3},
		{[]string{"apm", "tail", "d"}, 3},
		{[]string{"apm", "tail", "-0", "d"}, 2},
		{[]string{"apm", "tail", "x"}, 2},
		{[]string{"apm", "tail", "-x"}, 2},
	} {
		ret := eval(test.cmd, conn, state)
		assert.Equal(t, test.expectedLines, strings.Count(ret, "\n"))
	}
}
