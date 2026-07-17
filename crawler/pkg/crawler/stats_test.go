package crawler

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestStatsRecordVisitAccumulatesStatusCodes(t *testing.T) {
	s := NewStats()
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 5)
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 3)
	s.RecordVisit("example.com", 404, time.Millisecond, nil, 0)

	if s.statusCodes[200] != 2 {
		t.Errorf("expected 2 200s, got %d", s.statusCodes[200])
	}
	if s.statusCodes[404] != 1 {
		t.Errorf("expected 1 404, got %d", s.statusCodes[404])
	}
}

func TestStatsRecordVisitAccumulatesDuration(t *testing.T) {
	s := NewStats()
	s.RecordVisit("example.com", 200, 100*time.Millisecond, nil, 0)
	s.RecordVisit("example.com", 200, 200*time.Millisecond, nil, 0)

	if s.fetchDurCount != 2 {
		t.Errorf("expected fetchDurCount=2, got %d", s.fetchDurCount)
	}
	if s.fetchDurSum != 300*time.Millisecond {
		t.Errorf("expected fetchDurSum=300ms, got %v", s.fetchDurSum)
	}
}

func TestStatsRecordVisitDeduplicatesDomains(t *testing.T) {
	s := NewStats()
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 0)
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 0)
	s.RecordVisit("other.com", 200, time.Millisecond, nil, 0)

	if got := len(s.uniqueDomains); got != 2 {
		t.Errorf("expected 2 unique domains, got %d", got)
	}
}

func TestStatsRecordVisitTracksLastModifiedWhenPresent(t *testing.T) {
	s := NewStats()
	mod := time.Now().Add(-24 * time.Hour)
	s.RecordVisit("example.com", 200, time.Millisecond, &mod, 0)

	if s.lastModCount != 1 {
		t.Errorf("expected lastModCount=1, got %d", s.lastModCount)
	}
	if s.lastModSum <= 0 {
		t.Errorf("expected positive lastModSum, got %v", s.lastModSum)
	}
}

func TestStatsRecordVisitSkipsLastModifiedWhenAbsent(t *testing.T) {
	s := NewStats()
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 0)

	if s.lastModCount != 0 {
		t.Errorf("expected lastModCount=0, got %d", s.lastModCount)
	}
}

func TestStatsRecordErrorIncrementsErrors(t *testing.T) {
	s := NewStats()
	s.RecordError()
	s.RecordError()

	if s.errors != 2 {
		t.Errorf("expected errors=2, got %d", s.errors)
	}
}

func TestStatsLogOmitsContentAgeWhenNoLastModified(t *testing.T) {
	s := NewStats()
	s.RecordVisit("example.com", 200, time.Millisecond, nil, 0)

	rec := captureStatsLog(t, s)

	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "avg_content_age" {
			t.Error("expected avg_content_age to be omitted when no Last-Modified headers seen")
		}
		return true
	})
}

func TestStatsLogIncludesContentAgeWhenLastModifiedPresent(t *testing.T) {
	s := NewStats()
	mod := time.Now().Add(-48 * time.Hour)
	s.RecordVisit("example.com", 200, time.Millisecond, &mod, 0)

	rec := captureStatsLog(t, s)

	var found bool
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "avg_content_age" {
			found = true
		}
		return true
	})
	if !found {
		t.Error("expected avg_content_age in 'crawl stats' log when Last-Modified headers seen")
	}
}

func TestStatsSnapshotMatchesLogFields(t *testing.T) {
	s := NewStats()
	mod := time.Now().Add(-48 * time.Hour)
	s.RecordVisit("example.com", 200, 100*time.Millisecond, &mod, 3)
	s.RecordVisit("example.com", 404, 200*time.Millisecond, nil, 0)
	s.RecordError()

	snap := s.Snapshot(7)

	if snap.PagesVisited != 2 {
		t.Errorf("expected PagesVisited=2, got %d", snap.PagesVisited)
	}
	if snap.Errors != 1 {
		t.Errorf("expected Errors=1, got %d", snap.Errors)
	}
	if snap.UniqueDomains != 1 {
		t.Errorf("expected UniqueDomains=1, got %d", snap.UniqueDomains)
	}
	if snap.TotalOutlinks != 3 {
		t.Errorf("expected TotalOutlinks=3, got %d", snap.TotalOutlinks)
	}
	if snap.FrontierRemaining != 7 {
		t.Errorf("expected FrontierRemaining=7, got %d", snap.FrontierRemaining)
	}
	if snap.Status2xx != 1 || snap.Status4xx != 1 {
		t.Errorf("expected 1 2xx and 1 4xx, got 2xx=%d 4xx=%d", snap.Status2xx, snap.Status4xx)
	}
	if snap.AvgFetchMs != 150 {
		t.Errorf("expected AvgFetchMs=150, got %d", snap.AvgFetchMs)
	}
	if !snap.HasContentAge {
		t.Error("expected HasContentAge=true when a Last-Modified header was seen")
	}

	rec := captureStatsLog(t, s)
	var loggedPagesVisited int64
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "pages_visited" {
			loggedPagesVisited = a.Value.Int64()
		}
		return true
	})
	if loggedPagesVisited != int64(snap.PagesVisited) {
		t.Errorf("expected Log's pages_visited to match Snapshot's PagesVisited=%d, got %d", snap.PagesVisited, loggedPagesVisited)
	}
}

// captureStatsLog calls s.Log and returns the captured "crawl stats" slog.Record.
func captureStatsLog(t *testing.T, s *Stats) slog.Record {
	t.Helper()
	var mu sync.Mutex
	var records []slog.Record
	h := &captureHandler{capture: func(r slog.Record) {
		mu.Lock()
		records = append(records, r)
		mu.Unlock()
	}}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(orig)

	s.Log(0)

	mu.Lock()
	defer mu.Unlock()
	for _, r := range records {
		if r.Message == "crawl stats" {
			return r
		}
	}
	t.Fatal("expected 'crawl stats' log record but none was emitted")
	return slog.Record{}
}

type captureHandler struct {
	capture func(slog.Record)
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.capture(r)
	return nil
}
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }
