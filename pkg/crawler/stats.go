package crawler

import (
	"log/slog"
	"sync"
	"time"
)

type Stats struct {
	mu            sync.Mutex
	startTime     time.Time
	statusCodes   map[int]int
	uniqueDomains map[string]struct{}
	totalOutlinks int
	errors        int
	fetchDurSum   time.Duration
	fetchDurCount int
	lastModSum    time.Duration
	lastModCount  int
}

func NewStats() *Stats {
	return &Stats{
		startTime:     time.Now(),
		statusCodes:   make(map[int]int),
		uniqueDomains: make(map[string]struct{}),
	}
}

func (s *Stats) RecordVisit(host string, status int, duration time.Duration, lastModified *time.Time, outlinks int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCodes[status]++
	s.uniqueDomains[host] = struct{}{}
	s.totalOutlinks += outlinks
	s.fetchDurSum += duration
	s.fetchDurCount++
	if lastModified != nil {
		age := time.Since(*lastModified)
		if age > 0 {
			s.lastModSum += age
			s.lastModCount++
		}
	}
}

func (s *Stats) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors++
}

func (s *Stats) Log(frontierRemaining int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := time.Since(s.startTime).Round(time.Millisecond)

	var pagesPerSec float64
	if secs := elapsed.Seconds(); secs > 0 {
		pagesPerSec = float64(s.fetchDurCount) / secs
	}

	var avgFetchMs int64
	if s.fetchDurCount > 0 {
		avgFetchMs = s.fetchDurSum.Milliseconds() / int64(s.fetchDurCount)
	}

	var status2xx, status3xx, status4xx, status5xx int
	for code, count := range s.statusCodes {
		switch code / 100 {
		case 2:
			status2xx += count
		case 3:
			status3xx += count
		case 4:
			status4xx += count
		case 5:
			status5xx += count
		}
	}

	attrs := []any{
		slog.String("elapsed", elapsed.String()),
		slog.Int("pages_visited", s.fetchDurCount),
		slog.Float64("pages_per_sec", pagesPerSec),
		slog.Int("errors", s.errors),
		slog.Int("unique_domains", len(s.uniqueDomains)),
		slog.Int("total_outlinks", s.totalOutlinks),
		slog.Int("frontier_remaining", frontierRemaining),
		slog.Int("status_2xx", status2xx),
		slog.Int("status_3xx", status3xx),
		slog.Int("status_4xx", status4xx),
		slog.Int("status_5xx", status5xx),
		slog.Int64("avg_fetch_ms", avgFetchMs),
	}

	if s.lastModCount > 0 {
		avgAge := s.lastModSum / time.Duration(s.lastModCount)
		attrs = append(attrs, slog.String("avg_content_age", avgAge.Round(time.Minute).String()))
	}

	slog.Info("crawl stats", attrs...)
}
