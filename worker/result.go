package worker

import (
	"time"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/numbers"
	"github.com/elastic/hey-apm/strcoll"

	"go.elastic.co/apm"
)

type Result struct {
	apm.TracerStats
	agent.TransportStats
	Start   time.Time
	End     time.Time
	Flushed time.Time
}

func (r Result) TransactionSuccess() float64 {
	return numbers.Perct(r.TransactionsSent, r.TransactionsDropped)
}

func (r Result) SpanSuccess() float64 {
	return numbers.Perct(r.SpansSent, r.SpansDropped)
}

func (r Result) ErrorSuccess() float64 {
	return numbers.Perct(r.ErrorsSent, r.ErrorsDropped)
}

func (r Result) EventsSent() uint64 {
	return r.ErrorsSent + r.SpansSent + r.TransactionsSent
}

func (r Result) ElapsedSeconds() float64 {
	return r.Flushed.Sub(r.Start).Seconds()
}

func (r Result) EventsSentPerSecond() float64 {
	return float64(r.EventsSent()) / r.ElapsedSeconds()
}

func (r Result) EventsAcceptedPerSecond() float64 {
	return float64(r.Accepted) / r.ElapsedSeconds()
}

func (r Result) EventSuccess() float64 {
	return numbers.Perct(r.Accepted, r.EventsSent())
}

func (r Result) SpansPerTransaction() float64 {
	return numbers.Div(r.SpansSent, r.TransactionsSent)
}

func (r Result) String() string {
	metrics := strcoll.NewTuples()

	metrics.Add("transactions sent", r.TransactionsSent)
	metrics.Add("transactions dropped", r.TransactionsDropped)
	if r.TransactionsSent+r.TransactionsDropped > 0 {
		metrics.Add(" - success %", r.TransactionSuccess())
		metrics.Add("spans sent", r.SpansSent)
		metrics.Add("spans dropped", r.SpansDropped)
		metrics.Add(" - success %", r.SpanSuccess())
		if r.TransactionsSent > 0 {
			metrics.Add("spans sent per transaction", r.SpansPerTransaction())
		}
	}
	metrics.Add("errors sent", r.ErrorsSent)
	metrics.Add("errors dropped", r.ErrorsDropped)
	if r.ErrorsSent+r.ErrorsDropped > 0 {
		metrics.Add(" - success %", r.ErrorSuccess())
	}
	if r.ElapsedSeconds() > 0 {
		metrics.Add("total events sent", r.EventsSent())
		metrics.Add(" - per second", r.EventsSentPerSecond())
		metrics.Add(" - accepted", int64(r.Accepted))
		if r.Accepted > 0 && r.EventsSent() != r.Accepted {
			metrics.Add("   - per second", r.EventsAcceptedPerSecond())
			metrics.Add("   - success %", r.ErrorSuccess())
		}
	}

	metrics.Add("total requests", r.NumRequests)
	metrics.Add("failed", r.Errors.SendStream)
	if len(r.TopErrors) > 0 {
		metrics.Add("server errors", r.TopErrors)
	}

	return metrics.Format(30)
}
