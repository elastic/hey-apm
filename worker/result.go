package worker

import (
	"time"

	"github.com/elastic/hey-apm/strcoll"

	"go.elastic.co/apm"
)

// Result holds stats captured from a Go agent plus timing information.
type Result struct {
	apm.TracerStats
	TransportStats
	Start   time.Time
	End     time.Time
	Flushed time.Time
}

func (r Result) EventsGenerated() uint64 {
	sent := r.EventsSent()
	return sent + r.ErrorsDropped + r.ErrorsDropped + r.TransactionsDropped
}

func (r Result) EventsSent() uint64 {
	return r.ErrorsSent + r.SpansSent + r.TransactionsSent
}

func (r Result) ElapsedSeconds() float64 {
	return r.Flushed.Sub(r.Start).Seconds()
}

func (r Result) String() string {
	metrics := strcoll.NewTuples()

	metrics.Add("transactions sent", r.TransactionsSent)
	metrics.Add("transactions dropped", r.TransactionsDropped)
	if total := r.TransactionsSent + r.TransactionsDropped; total > 0 {
		metrics.Add(" - success %", 100*float64(r.TransactionsSent)/float64(total))
	}

	metrics.Add("spans sent", r.SpansSent)
	metrics.Add("spans dropped", r.SpansDropped)
	if total := r.SpansSent + r.SpansDropped; total > 0 {
		metrics.Add(" - success %", 100*float64(r.SpansSent)/float64(total))
	}
	if r.TransactionsSent > 0 {
		metrics.Add("spans sent per transaction", float64(r.SpansSent)/float64(r.TransactionsSent))
	}

	metrics.Add("errors sent", r.ErrorsSent)
	metrics.Add("errors dropped", r.ErrorsDropped)
	if total := r.ErrorsSent + r.ErrorsDropped; total > 0 {
		metrics.Add(" - success %", 100*float64(r.ErrorsSent)/float64(total))
	}

	if elapsedSeconds := r.ElapsedSeconds(); elapsedSeconds > 0 {
		eventsSent := r.EventsSent()
		metrics.Add("total events sent", eventsSent)
		metrics.Add(" - per second", float64(eventsSent)/elapsedSeconds)
		if total := r.EventsGenerated(); total > 0 {
			metrics.Add(" - success %", 100*float64(eventsSent)/float64(total))
		}
		metrics.Add(" - accepted", r.EventsAccepted)
		metrics.Add("   - per second", float64(r.EventsAccepted)/elapsedSeconds)
	}
	metrics.Add("total requests", r.NumRequests)
	metrics.Add("failed", r.Errors.SendStream)
	if len(r.UniqueErrors) > 0 {
		metrics.Add("server errors", r.UniqueErrors)
	}

	return metrics.Format(30)
}
