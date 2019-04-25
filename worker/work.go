package worker

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/elastic/hey-apm/conv"
	"github.com/elastic/hey-apm/numbers"
	"github.com/elastic/hey-apm/out"

	"github.com/heptio/workgroup"

	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"
)

type Worker struct {
	*out.ApmLogger
	*apm.Tracer
	RunTimeout   time.Duration
	FlushTimeout time.Duration

	// not to be modified concurrently
	workgroup.Group
}

type metric struct {
	Name, Value string
}

type Report struct {
	Stats   []metric
	Start   time.Time
	End     time.Time
	Flushed time.Time
}

func (r *Report) add(metricName string, value interface{}) {
	r.Stats = append(r.Stats, metric{metricName, conv.StringOf(value)})
}

func (w *Worker) Work() (Report, error) {
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

	report := Report{Stats: make([]metric, 0)}
	report.Start = time.Now()
	err := w.Run()
	report.End = time.Now()
	w.flush()
	report.Flushed = time.Now()

	rs := w.Stats()
	report.add("transactions sent", rs.TransactionsSent)
	report.add("transactions dropped", rs.TransactionsDropped)
	if rs.TransactionsSent+rs.TransactionsDropped > 0 {
		report.add(" - success %", numbers.Perct(rs.TransactionsSent, rs.TransactionsDropped))
		report.add("spans sent", rs.SpansSent)
		report.add("spans dropped", rs.SpansDropped)
		report.add(" - success %", numbers.Perct(rs.SpansSent, rs.SpansDropped))
		if rs.TransactionsSent > 0 {
			report.add("spans sent per transaction", numbers.Div(rs.SpansSent, rs.TransactionsSent))
		}
	}
	report.add("errors sent", rs.ErrorsSent)
	report.add("errors dropped", rs.ErrorsDropped)
	if rs.ErrorsSent+rs.ErrorsDropped > 0 {
		report.add(" - success %", numbers.Perct(rs.ErrorsSent, rs.ErrorsDropped))
	}
	eventsSent := float64(rs.ErrorsSent + rs.SpansSent + rs.TransactionsSent)
	report.add("total events sent", eventsSent)
	report.add(" - per second", eventsSent/report.Flushed.Sub(report.Start).Seconds())
	report.add("failed", rs.Errors.SendStream)

	return report, err
}

func (w *Worker) flush() {
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

func (w *Worker) AddErrors(frequency time.Duration, limit, framesMin, framesMax int) {
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

func (w *Worker) AddTransactions(frequency time.Duration, limit, spanMin, spanMax int) {
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

// throttle converts a time ticker to a channel of things
func throttle(c <-chan time.Time) chan interface{} {
	throttle := make(chan interface{})
	go func() {
		for range c {
			throttle <- struct{}{}
		}
	}()
	return throttle
}
