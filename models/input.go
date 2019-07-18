package models

import (
	"time"
)

// Input holds all the parameters given to a load test work.
// Most parameters describe a workload pattern, other are required to create performance reports.
type Input struct {
	ApmServerUrl    string `json:"apm_url"`
	ApmServerSecret string `json:"-"`

	ElasticsearchUrl  string `json:"-"`
	ElasticsearchAuth string `json:"-"`

	ApmElasticsearchUrl  string `json:"elastic_url,omitempty"`
	ApmElasticsearchAuth string `json:"-"`

	RunTimeout   time.Duration `json:"run_timeout"`
	FlushTimeout time.Duration `json:"flush_timeout"`

	ServiceName    string `json:"service_name"`

	TransactionFrequency time.Duration `json:"transaction_generation_frequency"`
	TransactionLimit     int           `json:"transaction_generation_limit"`

	SpanMaxLimit int `json:"spans_generated_max_limit"`
	SpanMinLimit int `json:"spans_generated_min_limit"`

	ErrorFrequency     time.Duration `json:"error_generation_frequency"`
	ErrorLimit         int           `json:"error_generation_limit"`
	ErrorFrameMaxLimit int           `json:"error_generation_frames_max_limit"`
	ErrorFrameMinLimit int           `json:"error_generation_frames_min_limit"`
}

type Wrap struct {
	Input
}

func (w Wrap) WithErrors(limit int, freq time.Duration) Wrap {
	w.ErrorLimit = limit
	w.ErrorFrequency = freq
	return w
}

func (w Wrap) WithFrames(f int) Wrap {
	w.ErrorFrameMaxLimit = f
	w.ErrorFrameMinLimit = f
	return w
}

func (w Wrap) WithTransactions(limit int, freq time.Duration) Wrap {
	w.TransactionLimit = limit
	w.TransactionFrequency = freq
	return w
}

func (w Wrap) WithSpans(s int) Wrap {
	w.SpanMaxLimit = s
	w.SpanMinLimit = s
	return w
}
