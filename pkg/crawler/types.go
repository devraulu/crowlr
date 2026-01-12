package crawler

import "github.com/devraulu/crowlr/pkg/storage"

type Outlink struct {
	Normalized string
	Original   string
}

type CrawlResult struct {
	URL      string
	Error    error
	PageData *storage.Page
	Outlinks []Outlink
}
