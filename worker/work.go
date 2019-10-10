package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/elastic/hey-apm/internal/heptio/workgroup"

	"github.com/elastic/hey-apm/agent"

	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"
)

type worker struct {
	*apmLogger
	*agent.Tracer
	RunTimeout   time.Duration
	FlushTimeout time.Duration

	// not to be modified concurrently
	workgroup.Group
}

// work uses the Go agent API to generate events and send them to apm-server.
func (w *worker) work() (Result, error) {
	if w.RunTimeout > 0 {
		w.Add(func(done <-chan struct{}) error {
			select {
			case <-done:
				return nil
			case <-time.After(w.RunTimeout):
				return nil // time expired
			}
		})
	}

	result := Result{}
	result.Start = time.Now()
	err := w.Run()
	result.End = time.Now()
	w.flush()
	result.Flushed = time.Now()
	result.TracerStats = w.Stats()
	result.TransportStats = *w.TransportStats

	return result, err
}

// flush ensures that the entire workload defined is pushed to the apm-server, within the worker timeout limit.
func (w *worker) flush() {
	flushed := make(chan struct{})
	go func() {
		w.Flush(nil)
		close(flushed)
	}()

	flushWait := time.After(w.FlushTimeout)
	if w.FlushTimeout == 0 {
		flushWait = make(<-chan time.Time)
	}
	select {
	case <-flushed:
	case <-flushWait:
		// give up waiting for flush
		w.Errorf("timed out waiting for flush to complete")
	}
	w.Close()
}

type generatedErr struct {
	frames int
}

func (e *generatedErr) Error() string {
	plural := "s"
	if e.frames == 1 {
		plural = ""
	}
	return fmt.Sprintf("Generated error with %d stacktrace frame%s", e.frames, plural)
}

// must be public for apm agent to use it - https://www.elastic.co/guide/en/apm/agent/go/current/api.html#error-api
func (e *generatedErr) StackTrace() []stacktrace.Frame {
	st := make([]stacktrace.Frame, e.frames)
	for i := 0; i < e.frames; i++ {
		st[i] = stacktrace.Frame{
			File:     "fake.go",
			Function: "oops",
			Line:     i + 100,
		}
	}
	return st
}

func (w *worker) addErrors(frequency time.Duration, limit, framesMin, framesMax int) {
	if limit <= 0 {
		return
	}
	t := throttle(time.NewTicker(frequency).C)
	w.Add(func(done <-chan struct{}) error {
		var count int
		for count < limit {
			select {
			case <-done:
				return nil
			case <-t:
			}

			w.Tracer.NewError(&generatedErr{frames: rand.Intn(framesMax-framesMin+1) + framesMin}).Send()
			count++
		}
		return nil
	})
}

func (w *worker) addTransactions(frequency time.Duration, limit, spanMin, spanMax int) {
	if limit <= 0 {
		return
	}
	t := throttle(time.NewTicker(frequency).C)
	generateSpan := func(ctx context.Context) {
		span, ctx := apm.StartSpan(ctx, "I'm a span", "gen.era.ted")
		span.End()
	}

	generator := func(done <-chan struct{}) error {
		var count int
		for count < limit {
			select {
			case <-done:
				return nil
			case <-t:
			}

			tx := w.Tracer.StartTransaction("generated", "gen")
			ctx := apm.ContextWithTransaction(context.Background(), tx)
			var wg sync.WaitGroup
			spanCount := rand.Intn(spanMax-spanMin+1) + spanMin
			for i := 0; i < spanCount; i++ {
				wg.Add(1)
				go func() {
					generateSpan(ctx)
					wg.Done()
				}()
			}
			wg.Wait()
			tx.Context.SetTag("spans", strconv.Itoa(spanCount))
			tx.End()
			count++
		}
		return nil
	}
	w.Add(generator)
}

func (w *worker) addSignalHandling() {
	w.Add(func(done <-chan struct{}) error {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		select {
		case <-done:
			return nil
		case sig := <-c:
			return errors.New(sig.String())
		}
	})
}

// throttle converts a time ticker to a channel of things.
func throttle(c <-chan time.Time) chan interface{} {
	throttle := make(chan interface{})
	go func() {
		for range c {
			throttle <- struct{}{}
		}
	}()
	return throttle
}
