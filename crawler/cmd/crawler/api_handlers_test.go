package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/jobs"
)

func newTestAPIHandler(t *testing.T, app *testApp) *apiHandler {
	t.Helper()
	records := jobs.NewPostgresRecordStore(app.db)
	mgr := jobs.NewManager(
		crawler.NewHTTPFetcher(&http.Client{}),
		app.store,
		records,
		jobs.Defaults{Workers: 1},
	)
	return &apiHandler{
		mgr: mgr,
		cfg: &config.Config{},
		ctx: t.Context(),
	}
}

func newAPIMux(h *apiHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.createCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.getCrawl)
	mux.HandleFunc("GET /crawls", h.listCrawls)
	return mux
}

func TestAPICreateAndPollCrawl(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><a href="/page-a">Page A</a></body></html>`)
	})
	mux.HandleFunc("/page-a", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Page A</h1></body></html>`)
	})

	app := newTestApp(t, mux)
	h := newTestAPIHandler(t, app)
	apiServer := newAPIMux(h)
	apiHTTP := httptest.NewServer(apiServer)
	t.Cleanup(apiHTTP.Close)

	body, _ := json.Marshal(createCrawlRequest{Seeds: []string{app.server.URL + "/"}})
	resp, err := http.Post(apiHTTP.URL+"/crawls", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /crawls failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var created jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Status != jobs.StatusPending {
		t.Errorf("expected initial status pending, got %s", created.Status)
	}

	final := pollUntilTerminal(t, apiHTTP.URL, created.ID)
	if final.Status != jobs.StatusCompleted {
		t.Fatalf("expected status completed, got %s (err=%s)", final.Status, final.Error)
	}
	if final.Stats.PagesVisited != 2 {
		t.Errorf("expected 2 pages visited, got %d", final.Stats.PagesVisited)
	}

	var pageCount int
	if err := app.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM pages`).Scan(&pageCount); err != nil {
		t.Fatalf("query pages: %v", err)
	}
	if pageCount != 2 {
		t.Errorf("expected 2 rows in pages, got %d", pageCount)
	}

	var status string
	var statsJSON []byte
	if err := app.db.QueryRowContext(t.Context(),
		`SELECT status, stats FROM crawl_jobs WHERE id = $1`, created.ID,
	).Scan(&status, &statsJSON); err != nil {
		t.Fatalf("query crawl_jobs: %v", err)
	}
	if status != string(jobs.StatusCompleted) {
		t.Errorf("expected crawl_jobs.status=completed, got %q", status)
	}
	if len(statsJSON) == 0 || string(statsJSON) == "{}" {
		t.Errorf("expected non-empty stats JSON in crawl_jobs, got %s", statsJSON)
	}
}

func TestAPICreateCrawlInvalidJSON(t *testing.T) {
	app := newTestApp(t, http.NewServeMux())
	h := newTestAPIHandler(t, app)
	apiHTTP := httptest.NewServer(newAPIMux(h))
	t.Cleanup(apiHTTP.Close)

	resp, err := http.Post(apiHTTP.URL+"/crawls", "application/json", bytes.NewReader([]byte("{invalid")))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAPICreateCrawlAllInvalidSeeds(t *testing.T) {
	app := newTestApp(t, http.NewServeMux())
	h := newTestAPIHandler(t, app)
	apiHTTP := httptest.NewServer(newAPIMux(h))
	t.Cleanup(apiHTTP.Close)

	body, _ := json.Marshal(createCrawlRequest{Seeds: []string{"://nope"}})
	resp, err := http.Post(apiHTTP.URL+"/crawls", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAPICreateCrawlNoSeedsAndNoSeedsFile(t *testing.T) {
	app := newTestApp(t, http.NewServeMux())
	h := newTestAPIHandler(t, app)
	apiHTTP := httptest.NewServer(newAPIMux(h))
	t.Cleanup(apiHTTP.Close)

	resp, err := http.Post(apiHTTP.URL+"/crawls", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAPIGetUnknownCrawlReturns404(t *testing.T) {
	app := newTestApp(t, http.NewServeMux())
	h := newTestAPIHandler(t, app)
	apiHTTP := httptest.NewServer(newAPIMux(h))
	t.Cleanup(apiHTTP.Close)

	resp, err := http.Get(apiHTTP.URL + "/crawls/does-not-exist")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAPIListCrawls(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body></body></html>`)
	})
	app := newTestApp(t, mux)
	h := newTestAPIHandler(t, app)
	apiHTTP := httptest.NewServer(newAPIMux(h))
	t.Cleanup(apiHTTP.Close)

	body, _ := json.Marshal(createCrawlRequest{Seeds: []string{app.server.URL + "/"}})
	resp, err := http.Post(apiHTTP.URL+"/crawls", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	resp, err = http.Get(apiHTTP.URL + "/crawls")
	if err != nil {
		t.Fatalf("GET /crawls failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 job in list, got %d", len(list))
	}
}

func pollUntilTerminal(t *testing.T, apiBaseURL, id string) jobResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(apiBaseURL + "/crawls/" + id)
		if err != nil {
			t.Fatalf("GET /crawls/%s failed: %v", id, err)
		}
		var jr jobResponse
		if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
			resp.Body.Close()
			t.Fatalf("decode response: %v", err)
		}
		resp.Body.Close()
		if jr.Status == jobs.StatusCompleted || jr.Status == jobs.StatusFailed || jr.Status == jobs.StatusCanceled {
			return jr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for crawl %s to finish", id)
	return jobResponse{}
}
