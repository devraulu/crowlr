package storage

import (
	"context"
	"time"
)

type Page struct {
	ID           string
	Referrer     string
	RawURL       string
	URL          string
	Timestamp    time.Time
	LastModified *time.Time
	Title        string
	Content      string
	HTML         string
	StatusCode   int
	Outlinks     []string
}

type Sitemap struct {
	URL         string
	LastChecked time.Time
	StatusCode  int
	Content     string
}

type SearchResult struct {
	URL     string
	Title   string
	Snippet string
	Rank    float64
}

type SearchResponse struct {
	Results    []SearchResult
	TotalCount int
}

type Storage interface {
	SavePage(ctx context.Context, p Page) error
	SaveSitemap(ctx context.Context, s Sitemap) error
	Search(ctx context.Context, query string, limit int) (SearchResponse, error)
	Close() error
}
