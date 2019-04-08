package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/heptio/workgroup"

	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"
)

type worker struct {
	*apmLogger
	*apm.Tracer
	runTimeout time.Duration

	count struct {
		errors       int64
		transactions int64
		spans        int64
	}

	// not to be modified concurrently
	workgroup.Group
}

type Report struct {
	Count int
	Start time.Time
	End   time.Time
}

type sampler struct {
	count int64
}

func (s *sampler) Sample(apm.TraceContext) bool {
	atomic.AddInt64(&s.count, 1)
	return true
}

func (w *worker) Counts() (int, int, int) {
	return int(atomic.LoadInt64(&w.count.errors)),
		int(atomic.LoadInt64(&w.count.transactions)),
		int(atomic.LoadInt64(&w.count.spans))
}

func (w *worker) Work() (Report, error) {
	if w.runTimeout > 0 {
		w.Add(func(done <-chan struct{}) error {
			select {
			case <-done:
				return nil
			case <-time.After(w.runTimeout):
				return nil // time expired
			}
		})
	}

	var s sampler
	w.Tracer.SetSampler(&s)

	report := Report{}
	report.Start = time.Now()
	err := w.Run()
	report.End = time.Now()
	report.Count = int(atomic.LoadInt64(&s.count))
	return report, err
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

func (w *worker) addErrors(throttle <-chan interface{}, limit, framesMin, framesMax int) *worker {
	if limit <= 0 {
		return w
	}
	w.Add(func(done <-chan struct{}) error {
		for cnt := 0; cnt < limit; cnt++ {
			select {
			case <-done:
				return nil
			case <-throttle:
			}

			apm.DefaultTracer.NewError(&generatedErr{frames: rand.Intn(framesMax-framesMin+1) + framesMin}).Send()
			atomic.AddInt64(&w.count.errors, 1)
		}
		return nil
	})
	return w
}

func (w *worker) addTransactions(throttle <-chan interface{}, limit, spanMin, spanMax int) *worker {
	if limit <= 0 {
		return w
	}
	generateSpan := func(ctx context.Context) {
		span, ctx := apm.StartSpan(ctx, "I'm a span", "gen.era.ted")
		span.End()
	}

	generator := func(done <-chan struct{}) error {
		for cnt := 0; cnt < limit; cnt++ {
			select {
			case <-done:
				return nil
			case <-throttle:
			}

			tx := apm.DefaultTracer.StartTransaction("generated", "gen")
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
			atomic.AddInt64(&w.count.transactions, 1)
			atomic.AddInt64(&w.count.spans, int64(spanCount))
		}
		return nil
	}

	w.Add(generator)
	return w
}
