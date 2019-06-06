package es

import "github.com/elastic/hey-apm/models"

type SearchResult struct {
	Hits Hits `json:"hits"`
}

type Hits struct {
	Hits []ActualHit `json:"hits"`
}

type ActualHit struct {
	Id     string        `json:"_id"`
	Source models.Report `json:"_source"`
}
