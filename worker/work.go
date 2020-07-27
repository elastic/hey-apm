package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/elastic/hey-apm/internal/heptio/workgroup"

	"github.com/elastic/hey-apm/agent"

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
	defer w.Close()

	ctx := context.Background()
	if w.FlushTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.FlushTimeout)
		defer cancel()
	}
	w.Flush(ctx.Done())
	if ctx.Err() != nil {
		w.Errorf("timed out waiting for flush to complete")
	}
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
	w.Add(func(done <-chan struct{}) error {
		ticker := time.NewTicker(frequency)
		defer ticker.Stop()
		for i := 0; i < limit; i++ {
			select {
			case <-done:
				return nil
			case <-ticker.C:
			}
			w.Tracer.NewError(&generatedErr{frames: rand.Intn(framesMax-framesMin+1) + framesMin}).Send()
		}
		return nil
	})
}

func (w *worker) addTransactions(frequency time.Duration, limit, spanMin, spanMax int) {
	if limit <= 0 {
		return
	}
	generator := func(done <-chan struct{}) error {
		ticker := time.NewTicker(frequency)
		defer ticker.Stop()
		for i := 0; i < limit; i++ {
			select {
			case <-done:
				return nil
			case <-ticker.C:
			}
			tx := w.Tracer.StartTransaction("generated", "gen")
			spanCount := rand.Intn(spanMax-spanMin+1) + spanMin
			for i := 0; i < spanCount; i++ {
				tx.StartSpan("I'm a span", "gen.era.ted", nil).End()
			}
			tx.Context.SetTag("spans", strconv.Itoa(spanCount))
			tx.End()
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
