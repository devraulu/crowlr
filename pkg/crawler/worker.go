package crawler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	frontier "github.com/devraulu/crowlr/pkg"
	"github.com/devraulu/crowlr/pkg/process"
	"github.com/devraulu/crowlr/pkg/storage"
)

func (c *Crawler) worker(ctx context.Context, id int, jobs <-chan frontier.Candidate, results chan<- CrawlResult) {
	slog.Info("worker started", "id", id)
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			slog.Debug("worker received job", slog.Int("id", id), slog.Any("job", job))
			results <- c.fetchAndProcess(job)
		}
	}
}

func (c *Crawler) fetchAndProcess(job frontier.Candidate) CrawlResult {
	res := CrawlResult{
		URL: job.Normalized,
	}

	req, err := http.NewRequest("GET", job.Normalized, nil)
	if err != nil {
		res.Error = err
		return res
	}

	req.Header.Add("Accept", "text/html")
	req.Header.Add("User-Agent", c.cfg.Crawler.UserAgent)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Error = err
		return res
	}
	defer resp.Body.Close()

	if !validateHTMLContentTypeHeader(resp, "text/html") {
		return res
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err
		return res
	}

	if !validateBodyContentType(body, "text/html") {
		return res
	}

	extracted, err := process.ExtractLinks(bytes.NewReader(body), job.Normalized)
	if err != nil {
		res.Error = err
		return res
	}

	var outlinks []Outlink

	for _, absolute := range extracted.Outlinks {
		normalized, err := process.Normalize(absolute)
		if err == nil {
			outlinks = append(outlinks, Outlink{
				Normalized: normalized,
				Original:   absolute,
			})
		}
	}

	res.Outlinks = outlinks

	textContent, err := process.ExtractText(bytes.NewReader(body))
	if err != nil {
		slog.Warn("failed to extract text",
			slog.String("url", job.Normalized),
			slog.Any("err", err))

		textContent = ""
		// continue even if text extraction fails
	}

	var lastMod *time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := http.ParseTime(lm); err == nil {
			lastMod = &t
		}
	}

	storageOutlinks := make([]string, len(outlinks))
	for i, o := range outlinks {
		storageOutlinks[i] = o.Normalized
	}

	res.PageData = &storage.Page{
		Referrer:     job.Referrer,
		RawURL:       job.Original,
		URL:          job.Normalized,
		Timestamp:    time.Now(),
		LastModified: lastMod,
		Title:        extracted.Title,
		Content:      textContent,
		HTML:         string(body),
		StatusCode:   resp.StatusCode,
		Outlinks:     storageOutlinks,
	}

	return res
}

func validateHTMLContentTypeHeader(resp *http.Response, contentType string) bool {
	header := resp.Header.Get("Content-Type")

	return strings.Contains(strings.ToLower(header), contentType)
}

func validateBodyContentType(body []byte, contentType string) bool {
	return strings.HasPrefix(http.DetectContentType(body), contentType)
}
