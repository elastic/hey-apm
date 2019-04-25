package worker

import (
	"fmt"
	"time"

	"github.com/elastic/hey-apm/strcoll"

	"github.com/elastic/hey-apm/agent"
	"github.com/elastic/hey-apm/numbers"
	"go.elastic.co/apm"
)

type Report struct {
	apm.TracerStats
	agent.TransportStats
	Start   time.Time
	End     time.Time
	Flushed time.Time
}

func (r Report) TransactionSuccess() float64 {
	return numbers.Perct(r.TransactionsSent, r.TransactionsDropped)
}

func (r Report) SpanSuccess() float64 {
	return numbers.Perct(r.SpansSent, r.SpansDropped)
}

func (r Report) ErrorSuccess() float64 {
	return numbers.Perct(r.ErrorsSent, r.ErrorsDropped)
}

func (r Report) EventsSent() uint64 {
	return r.ErrorsSent + r.SpansSent + r.TransactionsSent
}

func (r Report) ElapsedSeconds() float64 {
	return r.Flushed.Sub(r.Start).Seconds()
}

func (r Report) EventsSentPerSecond() float64 {
	return float64(r.EventsSent()) / r.ElapsedSeconds()
}

func (r Report) EventsAcceptedPerSecond() float64 {
	return float64(r.Accepted) / r.ElapsedSeconds()
}

func (r Report) EventSuccess() float64 {
	return numbers.Perct(r.Accepted, r.EventsSent())
}

func (r Report) SpansPerTransaction() float64 {
	return numbers.Div(r.SpansSent, r.TransactionsSent)
}

func (r Report) Print() {
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

	fmt.Println(metrics.Format(30))
}
