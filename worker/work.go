package worker

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"go.elastic.co/apm"
	"go.elastic.co/apm/stacktrace"
)

type worker struct {
	stop   <-chan struct{} // graceful shutdown
	logger *apmLogger
	tracer *tracer

	ErrorFrequency     time.Duration
	ErrorLimit         int
	ErrorFrameMinLimit int
	ErrorFrameMaxLimit int

	TransactionFrequency time.Duration
	TransactionLimit     int
	SpanMinLimit         int
	SpanMaxLimit         int

	RunTimeout   time.Duration
	FlushTimeout time.Duration
}

// work uses the Go agent API to generate events and send them to apm-server.
func (w *worker) work(ctx context.Context) (Result, error) {
	var runTimerC <-chan time.Time
	if w.RunTimeout > 0 {
		runTimer := time.NewTimer(w.RunTimeout)
		defer runTimer.Stop()
		runTimerC = runTimer.C
	}

	var errorTicker, transactionTicker maybeTicker
	if w.ErrorFrequency > 0 && w.ErrorLimit > 0 {
		errorTicker.Start(w.ErrorFrequency)
		defer errorTicker.Stop()
	}
	if w.TransactionFrequency > 0 && w.TransactionLimit > 0 {
		transactionTicker.Start(w.TransactionFrequency)
		defer transactionTicker.Stop()
	}

	result := Result{Start: time.Now()}
	var done bool
	for !done {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-w.stop:
			done = true
		case <-runTimerC:
			done = true
		case <-errorTicker.C:
			w.sendError()
			w.ErrorLimit--
			if w.ErrorLimit == 0 {
				errorTicker.Stop()
			}
		case <-transactionTicker.C:
			w.sendTransaction()
			w.TransactionLimit--
			if w.TransactionLimit == 0 {
				transactionTicker.Stop()
			}
		}
	}

	result.End = time.Now()
	w.flush()
	result.Flushed = time.Now()
	result.TracerStats = w.tracer.Stats()
	result.TransportStats = w.tracer.TransportStats()
	return result, nil
}

func (w *worker) sendError() {
	err := &generatedErr{frames: randRange(w.ErrorFrameMinLimit, w.ErrorFrameMaxLimit)}
	w.tracer.NewError(err).Send()
}

func (w *worker) sendTransaction() {
	tx := w.tracer.StartTransaction("generated", "gen")
	defer tx.End()
	spanCount := randRange(w.SpanMinLimit, w.SpanMaxLimit)
	sendSpans(tx, spanCount)
	tx.Context.SetTag("spans", strconv.Itoa(spanCount))
}

func sendSpans(tx *apm.Transaction, n int) {
	// Send spans in a separate goroutine, to ensure we keep
	// the number of stack frames stable despite changes to
	// hey-apm.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			tx.StartSpan("I'm a span", "gen.era.ted", nil).End()
		}
	}()
	<-done
}

func randRange(min, max int) int {
	return min + rand.Intn(max-min+1)
}

// flush ensures that the entire workload defined is pushed to the apm-server, within the worker timeout limit.
func (w *worker) flush() {
	defer w.tracer.Close()

	ctx := context.Background()
	if w.FlushTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.FlushTimeout)
		defer cancel()
	}
	w.tracer.Flush(ctx.Done())
	if ctx.Err() != nil {
		w.logger.Errorf("timed out waiting for flush to complete")
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

type maybeTicker struct {
	ticker *time.Ticker
	C      <-chan time.Time
}

func (t *maybeTicker) Start(d time.Duration) {
	t.ticker = time.NewTicker(d)
	t.C = t.ticker.C
}

func (t *maybeTicker) Stop() {
	t.ticker.Stop()
	t.C = nil
}
