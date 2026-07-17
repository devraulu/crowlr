package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/jobs"
)

type apiHandler struct {
	mgr *jobs.Manager
	cfg *config.Config
	ctx context.Context
}

type createCrawlRequest struct {
	Seeds        []string `json:"seeds,omitempty"`
	Limit        *int     `json:"limit,omitempty"`
	Workers      *int     `json:"workers,omitempty"`
	Delay        *string  `json:"delay,omitempty"`
	FetchTimeout *string  `json:"fetch_timeout,omitempty"`
	UserAgent    *string  `json:"user_agent,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type statsDTO struct {
	ElapsedMs         int64   `json:"elapsed_ms"`
	PagesVisited      int     `json:"pages_visited"`
	PagesPerSec       float64 `json:"pages_per_sec"`
	Errors            int     `json:"errors"`
	UniqueDomains     int     `json:"unique_domains"`
	TotalOutlinks     int     `json:"total_outlinks"`
	FrontierRemaining int     `json:"frontier_remaining"`
	Status2xx         int     `json:"status_2xx"`
	Status3xx         int     `json:"status_3xx"`
	Status4xx         int     `json:"status_4xx"`
	Status5xx         int     `json:"status_5xx"`
	AvgFetchMs        int64   `json:"avg_fetch_ms"`
	AvgContentAgeMs   *int64  `json:"avg_content_age_ms,omitempty"`
}

type jobResponse struct {
	ID         string      `json:"id"`
	Status     jobs.Status `json:"status"`
	SeedsCount int         `json:"seeds_count"`
	CreatedAt  time.Time   `json:"created_at"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Error      string      `json:"error,omitempty"`
	Stats      statsDTO    `json:"stats"`
}

func toJobResponse(s jobs.Snapshot) jobResponse {
	stats := statsDTO{
		ElapsedMs:         s.Stats.Elapsed.Milliseconds(),
		PagesVisited:      s.Stats.PagesVisited,
		PagesPerSec:       s.Stats.PagesPerSec,
		Errors:            s.Stats.Errors,
		UniqueDomains:     s.Stats.UniqueDomains,
		TotalOutlinks:     s.Stats.TotalOutlinks,
		FrontierRemaining: s.Stats.FrontierRemaining,
		Status2xx:         s.Stats.Status2xx,
		Status3xx:         s.Stats.Status3xx,
		Status4xx:         s.Stats.Status4xx,
		Status5xx:         s.Stats.Status5xx,
		AvgFetchMs:        s.Stats.AvgFetchMs,
	}
	if s.Stats.HasContentAge {
		ms := s.Stats.AvgContentAge.Milliseconds()
		stats.AvgContentAgeMs = &ms
	}

	return jobResponse{
		ID:         s.ID,
		Status:     s.Status,
		SeedsCount: s.SeedsCount,
		CreatedAt:  s.CreatedAt,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Error:      s.Err,
		Stats:      stats,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func (h *apiHandler) createCrawl(w http.ResponseWriter, r *http.Request) {
	var req createCrawlRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}

	var seeds []crawler.Link
	var err error
	if len(req.Seeds) > 0 {
		seeds, err = crawler.ParseSeeds(req.Seeds)
	} else {
		seeds, err = crawler.LoadSeedsFile(h.cfg.Crawler.SeedsFile)
	}
	if err != nil {
		if errors.Is(err, crawler.ErrNoSeeds) {
			writeError(w, http.StatusBadRequest, "no valid seeds provided")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to load seeds: "+err.Error())
		return
	}

	ov, err := parseOverrides(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	snap, err := h.mgr.Start(h.ctx, seeds, ov)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start crawl: "+err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, toJobResponse(snap))
}

func (h *apiHandler) getCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snap, err := h.mgr.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "crawl not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get crawl: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toJobResponse(snap))
}

func (h *apiHandler) listCrawls(w http.ResponseWriter, r *http.Request) {
	all, err := h.mgr.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list crawls: "+err.Error())
		return
	}
	resp := make([]jobResponse, len(all))
	for i, s := range all {
		resp[i] = toJobResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseOverrides(req createCrawlRequest) (jobs.Overrides, error) {
	var ov jobs.Overrides
	ov.UserAgent = req.UserAgent
	ov.Workers = req.Workers
	ov.CrawlLimit = req.Limit
	if req.Delay != nil {
		d, err := time.ParseDuration(*req.Delay)
		if err != nil {
			return ov, fmt.Errorf("invalid delay %q: %w", *req.Delay, err)
		}
		ov.CrawlDelay = &d
	}
	if req.FetchTimeout != nil {
		d, err := time.ParseDuration(*req.FetchTimeout)
		if err != nil {
			return ov, fmt.Errorf("invalid fetch_timeout %q: %w", *req.FetchTimeout, err)
		}
		ov.FetchTimeout = &d
	}
	return ov, nil
}
