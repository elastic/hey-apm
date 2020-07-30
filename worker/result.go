package worker

import (
	"bytes"
	"fmt"
	"text/tabwriter"
	"time"

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
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 30, 8, 0, '.', 0)
	add := func(key, format string, value interface{}) {
		fmt.Fprintf(tw, "%s \t "+format+"\n", key, value)
	}

	add("transactions sent", "%d", r.TransactionsSent)
	add("transactions dropped", "%d", r.TransactionsDropped)
	if total := r.TransactionsSent + r.TransactionsDropped; total > 0 {
		add(" - success %", "%.2f", 100*float64(r.TransactionsSent)/float64(total))
	}

	add("spans sent", "%d", r.SpansSent)
	add("spans dropped", "%d", r.SpansDropped)
	if total := r.SpansSent + r.SpansDropped; total > 0 {
		add(" - success %", "%.2f", 100*float64(r.SpansSent)/float64(total))
	}
	if r.TransactionsSent > 0 {
		add("spans sent per transaction", "%.2f", float64(r.SpansSent)/float64(r.TransactionsSent))
	}

	add("errors sent", "%d", r.ErrorsSent)
	add("errors dropped", "%d", r.ErrorsDropped)
	if total := r.ErrorsSent + r.ErrorsDropped; total > 0 {
		add(" - success %", "%.2f", 100*float64(r.ErrorsSent)/float64(total))
	}

	if elapsedSeconds := r.ElapsedSeconds(); elapsedSeconds > 0 {
		eventsSent := r.EventsSent()
		add("total events sent", "%d", eventsSent)
		add(" - per second", "%.2f", float64(eventsSent)/elapsedSeconds)
		if total := r.EventsGenerated(); total > 0 {
			add(" - success %", "%.2f", 100*float64(eventsSent)/float64(total))
		}
		add(" - accepted", "%d", r.EventsAccepted)
		add("   - per second", "%.2f", float64(r.EventsAccepted)/elapsedSeconds)
	}
	add("total requests", "%d", r.NumRequests)
	add("failed", "%d", r.Errors.SendStream)
	if len(r.UniqueErrors) > 0 {
		add("server errors", "%d", r.UniqueErrors)
	}

	tw.Flush()
	return buf.String()
}
