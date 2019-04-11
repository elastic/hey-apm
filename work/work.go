package work

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/elastic/hey-apm/tracer"

	"github.com/elastic/hey-apm/out"
	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"

	"github.com/heptio/workgroup"
)

type Worker struct {
	// not to be modified concurrently
	workgroup.Group

	RunTimeout             time.Duration
	TransactionLimit       int
	TransactionFrequency   time.Duration
	MaxSpansPerTransaction int
	MinSpansPerTransaction int
	ErrorLimit             int
	ErrorFrequency         time.Duration
	MaxFramesPerError      int
	MinFramesPerError      int
}

type Report struct {
	Stats apm.TracerStats
	Start time.Time
	End   time.Time
}

func (w *Worker) Work(tracer *tracer.Tracer) (Report, error) {

	logger := out.NewApmLogger(log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile))

	if w.ErrorLimit > 0 {
		w.Add(errors(throttle(w.ErrorFrequency), tracer, w.ErrorLimit, w.MinFramesPerError, w.MaxFramesPerError))
	}
	if w.TransactionLimit > 0 {
		w.Add(transactions(throttle(w.TransactionFrequency), tracer, w.TransactionLimit, w.MinSpansPerTransaction, w.MaxSpansPerTransaction))
	}
	if w.RunTimeout > 0 {
		w.Add(timeout(w.RunTimeout))
	}

	logger.Debugf("start")
	defer logger.Debugf("finish")

	report := Report{Start: time.Now()}
	err := w.Run()
	tracer.FlushAll()
	report.End = time.Now()
	report.Stats = tracer.Stats()
	return report, err
}

// throttle converts a time ticker to a channel of things
func throttle(d time.Duration) chan interface{} {
	throttle := make(chan interface{})
	go func() {
		for range time.NewTicker(d).C {
			throttle <- struct{}{}
		}
	}()
	return throttle
}

type generator func(<-chan struct{}) error

func transactions(throttle <-chan interface{}, tracer *tracer.Tracer, limit, spanMin, spanMax int) generator {
	generateSpan := func(ctx context.Context) {
		span, ctx := apm.StartSpan(ctx, "I'm a span", "gen.era.ted")
		span.End()
	}
	return func(done <-chan struct{}) error {
		sent := 0
		for sent < limit {
			select {
			case <-done:
				return nil
			case <-throttle:
			}

			tx := tracer.StartTransaction("generated", "gen")
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

func errors(throttle <-chan interface{}, tracer *tracer.Tracer, limit, framesMin, framesMax int) generator {
	return func(done <-chan struct{}) error {
		sent := 0
		for sent < limit {
			select {
			case <-done:
				return nil
			case <-throttle:
			}
			tracer.NewError(&generatedErr{frames: rand.Intn(framesMax-framesMin+1) + framesMin}).Send()
			sent++
		}
		return nil
	}
}

func timeout(d time.Duration) generator {
	return func(done <-chan struct{}) error {
		select {
		case <-done:
			return nil
		case <-time.After(d):
			return nil // time expired
		}
	}
}
