package es

import "github.com/elastic/hey-apm/reports"

type SearchResult struct {
	Hits Hits `json:"hits"`
}

type Hits struct {
	Hits []ActualHit `json:"hits"`
}

type ActualHit struct {
	Id     string         `json:"_id"`
	Source reports.Report `json:"_source"`
}
