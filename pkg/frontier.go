package frontier

import (
	"log/slog"
	netUrl "net/url"
	"strings"
	"sync"
	"time"
)

type Candidate struct {
	Original   string
	Normalized string
	Referrer   string
}

type HostQueue struct {
	Host      string
	URLs      []string
	NextVisit time.Time
}

type SeenRecord struct {
	OriginalURL string
	Referrer    string
}

type Frontier struct {
	mu     sync.Mutex
	queues map[string]*HostQueue
	seen   map[string]SeenRecord
}

func NewFrontier() *Frontier {
	return &Frontier{
		queues: make(map[string]*HostQueue),
		seen:   make(map[string]SeenRecord),
	}
}

func (f *Frontier) Push(normalizedURL, originalURL, referrer string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.seen[normalizedURL]; ok {
		slog.Info("frontier duplicate, skipping", slog.String("url", normalizedURL), slog.String("original_url", originalURL))
		return
	}

	host, err := getHost(normalizedURL)
	if err != nil {
		slog.Error("frontier bad url", slog.String("url", normalizedURL), slog.Any("err", err))
		return
	}

	f.seen[normalizedURL] = SeenRecord{
		OriginalURL: originalURL,
		Referrer:    referrer,
	}

	hq, ok := f.queues[host]
	if !ok {
		hq = &HostQueue{
			Host: host,
		}
		f.queues[host] = hq
	}

	hq.URLs = append(hq.URLs, normalizedURL)
	slog.Debug("frontier push", slog.String("host", host), slog.String("url", normalizedURL), slog.Int("queue_len", len(hq.URLs)))
}

func (f *Frontier) Pop(defaultDelay time.Duration) (*Candidate, time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.queues) == 0 {
		return nil, 0
	}

	now := time.Now()
	var minWait time.Duration = -1

	for host, hq := range f.queues {
		if len(hq.URLs) == 0 {
			delete(f.queues, host)
			continue
		}

		if now.After(hq.NextVisit) {
			// found a ready host!
			url := hq.URLs[0]
			hq.URLs = hq.URLs[1:]
			hq.NextVisit = now.Add(defaultDelay)

			slog.Info("next candidate", slog.String("host", host), slog.String("url", url))
			seen := f.seen[url]
			referrer := seen.Referrer

			return &Candidate{
				Original:   seen.OriginalURL,
				Normalized: url,
				Referrer:   referrer,
			}, 0
		}

		wait := hq.NextVisit.Sub(now)
		if minWait == -1 || wait < minWait {
			minWait = wait
		}
	}

	return nil, minWait
}

func (f *Frontier) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, hq := range f.queues {
		count += len(hq.URLs)
	}
	return count
}

func getHost(str string) (string, error) {
	u, err := netUrl.Parse(str)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Hostname()), nil
}
