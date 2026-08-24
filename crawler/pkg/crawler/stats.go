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

func (s *Stats) RecordVisit(host string, status int, duration time.Duration, outlinks int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCodes[status]++
	s.uniqueDomains[host] = struct{}{}
	s.totalOutlinks += outlinks
	s.fetchDurSum += duration
	s.fetchDurCount++
}

func (s *Stats) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors++
}

type StatsSnapshot struct {
	Elapsed           time.Duration
	PagesVisited      int
	PagesPerSec       float64
	Errors            int
	UniqueDomains     int
	TotalOutlinks     int
	FrontierRemaining int
	Status2xx         int
	Status3xx         int
	Status4xx         int
	Status5xx         int
	AvgFetchMs        int64
	AvgContentAge     time.Duration
	HasContentAge     bool
}

func (s *Stats) Snapshot(frontierRemaining int) StatsSnapshot {
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

	snap := StatsSnapshot{
		Elapsed:           elapsed,
		PagesVisited:      s.fetchDurCount,
		PagesPerSec:       pagesPerSec,
		Errors:            s.errors,
		UniqueDomains:     len(s.uniqueDomains),
		TotalOutlinks:     s.totalOutlinks,
		FrontierRemaining: frontierRemaining,
		Status2xx:         status2xx,
		Status3xx:         status3xx,
		Status4xx:         status4xx,
		Status5xx:         status5xx,
		AvgFetchMs:        avgFetchMs,
	}

	if s.lastModCount > 0 {
		snap.AvgContentAge = (s.lastModSum / time.Duration(s.lastModCount)).Round(time.Minute)
		snap.HasContentAge = true
	}

	return snap
}

func (s *Stats) Log(frontierRemaining int) {
	snap := s.Snapshot(frontierRemaining)

	attrs := []any{
		slog.String("elapsed", snap.Elapsed.String()),
		slog.Int("pages_visited", snap.PagesVisited),
		slog.Float64("pages_per_sec", snap.PagesPerSec),
		slog.Int("errors", snap.Errors),
		slog.Int("unique_domains", snap.UniqueDomains),
		slog.Int("total_outlinks", snap.TotalOutlinks),
		slog.Int("frontier_remaining", snap.FrontierRemaining),
		slog.Int("status_2xx", snap.Status2xx),
		slog.Int("status_3xx", snap.Status3xx),
		slog.Int("status_4xx", snap.Status4xx),
		slog.Int("status_5xx", snap.Status5xx),
		slog.Int64("avg_fetch_ms", snap.AvgFetchMs),
	}

	if snap.HasContentAge {
		attrs = append(attrs, slog.String("avg_content_age", snap.AvgContentAge.String()))
	}

	slog.Info("crawl stats", attrs...)
}
