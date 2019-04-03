package main

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/heptio/workgroup"
	"go.elastic.co/apm"
)

type worker struct {
	*apmLogger
	*apm.Tracer
	runTimeout time.Duration

	// not to be modified concurrently
	workgroup.Group
}

type Report struct {
	Count int
	Start time.Time
	End   time.Time
}

type sampler struct {
	count int
}

func (s *sampler) Sample(apm.TraceContext) bool {
	s.count++
	return true
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
	report.Count = s.count
	return report, err
}

func (w *worker) addTransactions(limit, spanMin, spanMax int) *worker {
	if limit <= 0 {
		return w
	}
	generateSpan := func(ctx context.Context) {
		time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
		span, ctx := apm.StartSpan(ctx, "I'm a span", "gen.era.ted")
		defer span.End()
		r := rand.Intn(500)
		span.Context.SetTag("took_ms", strconv.Itoa(r))
		time.Sleep(time.Duration(r) * time.Millisecond)
	}

	generator := func(done <-chan struct{}) error {
		for cnt := 0; cnt < limit; cnt++ {
			select {
			case <-done:
				return nil
			default:
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
		}
		return nil
	}

	w.Add(generator)
	return w
}
