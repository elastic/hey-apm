package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/heptio/workgroup"

	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"
)

type worker struct {
	*apmLogger
	*apm.Tracer
	runTimeout   time.Duration
	flushTimeout time.Duration

	// not to be modified concurrently
	workgroup.Group
}

type pair struct {
	k, v string
}

type Report struct {
	Stats   []pair
	Start   time.Time
	End     time.Time
	Flushed time.Time
}

func (r *Report) add(metric string, value interface{}) {
	r.Stats = append(r.Stats, pair{metric, stringOf(value)})
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

	report := Report{Stats: make([]pair, 0)}
	report.Start = time.Now()
	err := w.Run()
	report.End = time.Now()
	w.flush()
	report.Flushed = time.Now()

	rs := w.Stats()
	report.add("transactions sent", rs.TransactionsSent)
	report.add("transactions dropped", rs.TransactionsDropped)
	if rs.TransactionsSent+rs.TransactionsDropped > 0 {
		report.add(" - success %", perct(rs.TransactionsSent, rs.TransactionsDropped))
		report.add("spans sent", rs.SpansSent)
		report.add("spans dropped", rs.SpansDropped)
		report.add(" - success %", perct(rs.SpansSent, rs.SpansDropped))
		if rs.TransactionsSent > 0 {
			report.add("spans sent per transaction", div(rs.SpansSent, rs.TransactionsSent))
		}
	}
	report.add("errors sent", rs.ErrorsSent)
	report.add("errors dropped", rs.ErrorsDropped)
	if rs.ErrorsSent+rs.ErrorsDropped > 0 {
		report.add(" - success %", perct(rs.ErrorsSent, rs.ErrorsDropped))
	}
	eventsSent := float64(rs.ErrorsSent + rs.SpansSent + rs.TransactionsSent)
	report.add("total events sent", eventsSent)
	report.add(" - per second", eventsSent/report.Flushed.Sub(report.Start).Seconds())
	report.add("failed", rs.Errors.SendStream)

	return report, err
}

func (w *worker) flush() {
	flushed := make(chan struct{})
	go func() {
		w.Flush(nil)
		close(flushed)
	}()

	flushWait := time.After(w.flushTimeout)
	if w.flushTimeout == 0 {
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

func perct(i1, i2 uint64) float64 {
	return div(i1*100, i1+i2)
}

func div(i1, i2 uint64) float64 {
	return float64(i1) / float64(i2)
}

func stringOf(v interface{}) string {
	switch v.(type) {
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%v", v)
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
		var sent int
		for sent < limit {
			select {
			case <-done:
				return nil
			case <-throttle:
			}

			apm.DefaultTracer.NewError(&generatedErr{frames: rand.Intn(framesMax-framesMin+1) + framesMin}).Send()
			sent++
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
		var sent int
		for sent < limit {
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
			sent++
		}
		return nil
	}

	w.Add(generator)
	return w
}
