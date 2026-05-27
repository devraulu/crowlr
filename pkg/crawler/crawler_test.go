package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCrawlerVisitsLink(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": "",
		"https://example.com/bar": "",
	}}
	link1, _ := NewLink("https://example.com/foo", WithReferrer("https://example.com/bar"))
	link2, _ := NewLink("https://example.com/bar")

	NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

	fetcher.assertTotalVisits(t, 2)
}

func TestCrawlerFindsNewLinks(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo":         htmlBody("Title", "https://domain.com/discovered-1", "https://domain.com/discovered-2"),
		"https://example.com/bar":         "",
		"https://domain.com/discovered-1": "",
		"https://domain.com/discovered-2": "",
	}}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com/bar")

	NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

	fetcher.assertTotalVisits(t, 4)
}

func TestCrawlerFailsOnEmptySeeds(t *testing.T) {
	err := NewCrawler(&mockFetcher{}, NullStore{}).Run(t.Context(), []Link{})
	if err == nil {
		t.Fatal("expected error, but got nil")
	}
}

func TestCrawlerDoesNotCrawlDuplicates(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": "",
	}}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com:80/foO")

	NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

	fetcher.assertTotalVisits(t, 1)
}

func TestShouldNotFetchTheSameHostBeforeDelay(t *testing.T) {
	const delay = 50 * time.Millisecond
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": "",
		"https://example.com/bar": "",
	}}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com/bar")

	NewCrawler(fetcher, NullStore{}, WithCrawlDelay(delay)).Run(t.Context(), []Link{link1, link2})

	fetcher.assertTotalVisits(t, 2)
	fetcher.assertVisitGap(t, "https://example.com/foo", "https://example.com/bar", delay)
}

func TestCrawlerShouldFetchNextAvailableHost(t *testing.T) {
	const delay = 50 * time.Millisecond
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": "",
		"https://example.com/bar": "",
		"https://baz.com/":        "",
	}}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com/bar")
	link3, _ := NewLink("https://baz.com/")

	NewCrawler(fetcher, NullStore{}, WithCrawlDelay(delay)).Run(t.Context(), []Link{link1, link2, link3})

	fetcher.assertTotalVisits(t, 3)
	fetcher.assertVisitedBefore(t, "https://baz.com/", "https://example.com/bar")
}

func TestCrawlerShouldRespectRobotsTxt(t *testing.T) {
	t.Run("robots.txt should be fetched", func(t *testing.T) {
		robotsURL := "https://domain.com/robots.txt"
		fetcher := &mockFetcher{pages: map[string]string{
			"https://domain.com/foo": "",
			"https://domain.com/cat": "",
			robotsURL:                "",
		}}
		link1, _ := NewLink("https://domain.com/foo")
		link2, _ := NewLink("https://domain.com/cat")

		NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

		fetcher.assertVisited(t, robotsURL)
	})

	t.Run("disallowed by robots.txt", func(t *testing.T) {
		fetcher := &mockFetcher{pages: map[string]string{
			"https://domain.com/foo":        "",
			"https://domain.com/cat":        "",
			"https://domain.com/robots.txt": robotsTxt("*", "/cat", "", ""),
		}}
		link1, _ := NewLink("https://domain.com/foo")
		link2, _ := NewLink("https://domain.com/cat")

		NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

		fetcher.assertNotVisited(t, "https://domain.com/cat")
	})

	t.Run("allowed by robots.txt", func(t *testing.T) {
		fetcher := &mockFetcher{pages: map[string]string{
			"https://domain.com/foo":        "",
			"https://domain.com/cat":        "",
			"https://domain.com/robots.txt": robotsTxt("*", "", "/cat", ""),
		}}
		link1, _ := NewLink("https://domain.com/foo")
		link2, _ := NewLink("https://domain.com/cat")

		NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

		fetcher.assertVisited(t, "https://domain.com/cat")
	})

	t.Run("disallowed user agent is not visited", func(t *testing.T) {
		fetcher := &mockFetcher{pages: map[string]string{
			"https://domain.com/foo":        "",
			"https://domain.com/cat":        "",
			"https://domain.com/robots.txt": robotsTxt("BadBot", "/", "", ""),
		}}
		link1, _ := NewLink("https://domain.com/foo")
		link2, _ := NewLink("https://domain.com/cat")

		NewCrawler(fetcher, NullStore{}, WithUserAgent("BadBot")).Run(t.Context(), []Link{link1, link2})

		fetcher.assertNotVisited(t, "https://domain.com/foo")
		fetcher.assertNotVisited(t, "https://domain.com/cat")
	})

	t.Run("robots.txt is fetched once per host", func(t *testing.T) {
		robotsURL := "https://domain.com/robots.txt"
		fetcher := &mockFetcher{pages: map[string]string{
			"https://domain.com/foo": "",
			"https://domain.com/cat": "",
			robotsURL:                robotsTxt("*", "", "", ""),
		}}
		link1, _ := NewLink("https://domain.com/foo")
		link2, _ := NewLink("https://domain.com/cat")

		NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

		fetcher.assertHitCount(t, robotsURL, 1)
	})
}

func TestSitemapLinksShouldBeCrawled(t *testing.T) {
	sitemapURL := "https://domain.com/sitemap.xml"
	fetcher := &mockFetcher{pages: map[string]string{
		"https://domain.com/foo":          "",
		"https://domain.com/cat":          "",
		"https://domain.com/robots.txt":   robotsTxt("*", "", "/cat", sitemapURL),
		sitemapURL:                        sampleXML,
		"http://www.example.net/?id=who":  "",
		"http://www.example.net/?id=what": "",
		"http://www.example.net/?id=how":  "",
	}}
	link1, _ := NewLink("https://domain.com/foo")
	link2, _ := NewLink("https://domain.com/cat")

	NewCrawler(fetcher, NullStore{}).Run(t.Context(), []Link{link1, link2})

	fetcher.assertVisited(t, sitemapURL)
	fetcher.assertVisited(t, "http://www.example.net/?id=who")
}

func TestCrawlerFetchesConcurrently(t *testing.T) {
	const fetchDelay = 100 * time.Millisecond
	fetcher := &mockFetcher{
		fetchDelay: fetchDelay,
		pages: map[string]string{
			"https://example.com/robots.txt": "",
			"https://example.com/1":          "",
			"https://example.com/2":          "",
			"https://example.com/3":          "",
			"https://example.com/4":          "",
		},
	}
	links := make([]Link, 4)
	for i := range 4 {
		links[i], _ = NewLink(fmt.Sprintf("https://example.com/%d", i+1))
	}

	start := time.Now()
	NewCrawler(fetcher, NullStore{}, WithWorkers(4)).Run(t.Context(), links)
	elapsed := time.Since(start)

	// robots.txt is fetched once (sequential, fetchDelay), then 4 pages fetch in parallel (~fetchDelay).
	// sequential would be robots + 4*pages = 5*fetchDelay.
	if elapsed > 3*fetchDelay {
		t.Errorf("expected concurrent fetching, took %v (want < %v)", elapsed, 3*fetchDelay)
	}
}

func TestCrawlerOnlyFetchesHTML(t *testing.T) {
	fetcher := &mockFetcher{
		pages: map[string]string{
			"https://example.com/page.html": "<html></html>",
			"https://example.com/file.pdf":  "%PDF-1.4 binary content",
		},
		responseHeaders: map[string]http.Header{
			"https://example.com/file.pdf": {"Content-Type": []string{"application/pdf"}},
		},
	}
	store := &mockStore{}
	htmlLink, _ := NewLink("https://example.com/page.html")
	pdfLink, _ := NewLink("https://example.com/file.pdf")

	NewCrawler(fetcher, store).Run(t.Context(), []Link{htmlLink, pdfLink})

	store.assertSaved(t, "https://example.com/page.html")
	store.assertNotSaved(t, "https://example.com/file.pdf")
}

func TestCrawlerTimesOutOnSlowResponses(t *testing.T) {
	const fetchDelay = 500 * time.Millisecond
	const timeout = 100 * time.Millisecond

	fetcher := &mockFetcher{
		fetchDelay: fetchDelay,
		pages:      map[string]string{"https://example.com/slow": ""},
	}
	link, _ := NewLink("https://example.com/slow")

	start := time.Now()
	NewCrawler(fetcher, NullStore{}, WithFetchTimeout(timeout)).Run(t.Context(), []Link{link})
	elapsed := time.Since(start)

	if elapsed > fetchDelay {
		t.Errorf("fetch was not cancelled by timeout: took %v, timeout was %v", elapsed, timeout)
	}
}

func TestCrawlerHasSpecifiedUserAgent(t *testing.T) {
	const ua = "crowlr-test/1.0"
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/": "",
	}}
	link, _ := NewLink("https://example.com/")

	NewCrawler(fetcher, NullStore{}, WithUserAgent(ua)).Run(t.Context(), []Link{link})

	fetcher.mu.Lock()
	got := fetcher.lastHeaders.Get("User-Agent")
	fetcher.mu.Unlock()

	if got != ua {
		t.Errorf("expected User-Agent %q, got %q", ua, got)
	}
}

func TestVisitReturnsTitle(t *testing.T) {
	title := "Example Title"
	link, _ := NewLink("https://example.com/")
	fetcher := &mockFetcher{
		pages: map[string]string{
			link.Normalized: htmlBody(title, ""),
		},
	}

	c := NewCrawler(fetcher, NullStore{})
	result := c.visit(t.Context(), link)
	if result.Title != title {
		t.Errorf("wanted result title to be %s, but got %s", title, result.Title)
	}
}

func TestVisitReturnsDuration(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/": "",
	}}
	c := NewCrawler(fetcher, NullStore{})
	link, _ := NewLink("https://example.com/")

	result := c.visit(t.Context(), link)

	if result.Duration <= 0 {
		t.Errorf("expected positive Duration, got %v", result.Duration)
	}
}

func TestVisitParsesLastModifiedHeader(t *testing.T) {
	mod := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	fetcher := &mockFetcher{
		pages: map[string]string{"https://example.com/": ""},
		responseHeaders: map[string]http.Header{
			"https://example.com/": {"Last-Modified": []string{mod.Format(http.TimeFormat)}},
		},
	}
	c := NewCrawler(fetcher, NullStore{})
	link, _ := NewLink("https://example.com/")

	result := c.visit(t.Context(), link)

	if result.LastModified == nil {
		t.Fatal("expected LastModified to be set, got nil")
	}
	if !result.LastModified.Equal(mod) {
		t.Errorf("expected LastModified=%v, got %v", mod, *result.LastModified)
	}
}

func TestVisitReturnsNilLastModifiedWhenHeaderAbsent(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/": "",
	}}
	c := NewCrawler(fetcher, NullStore{})
	link, _ := NewLink("https://example.com/")

	result := c.visit(t.Context(), link)

	if result.LastModified != nil {
		t.Errorf("expected LastModified to be nil, got %v", result.LastModified)
	}
}

func TestCrawlerPopulatesStatsAfterRun(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": "",
		"https://example.com/bar": "",
	}}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com/bar")
	c := NewCrawler(fetcher, NullStore{})

	c.Run(t.Context(), []Link{link1, link2})

	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()
	if c.stats.fetchDurCount != 2 {
		t.Errorf("expected 2 pages visited in stats, got %d", c.stats.fetchDurCount)
	}
	if c.stats.statusCodes[200] != 2 {
		t.Errorf("expected 2 status-200 in stats, got %d", c.stats.statusCodes[200])
	}
	if len(c.stats.uniqueDomains) != 1 {
		t.Errorf("expected 1 unique domain, got %d", len(c.stats.uniqueDomains))
	}
}

func TestCrawlerRecordsErrorsInStats(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/ok":  "",
		"https://example.com/bad": "", // will time out
	}}
	fetcher.errorURLs = map[string]bool{"https://example.com/bad": true}
	link1, _ := NewLink("https://example.com/ok")
	link2, _ := NewLink("https://example.com/bad")
	c := NewCrawler(fetcher, NullStore{})

	c.Run(t.Context(), []Link{link1, link2})

	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()
	if c.stats.errors != 1 {
		t.Errorf("expected 1 error in stats, got %d", c.stats.errors)
	}
}

func TestCrawlerStoresResults(t *testing.T) {
	fetcher := &mockFetcher{pages: map[string]string{
		"https://example.com/foo": htmlBody("https://example.com/bar"),
		"https://example.com/bar": "",
	}}
	store := &mockStore{}
	link1, _ := NewLink("https://example.com/foo")
	link2, _ := NewLink("https://example.com/bar")

	NewCrawler(fetcher, store).Run(t.Context(), []Link{link1, link2})

	store.assertSaved(t, "https://example.com/foo")
	store.assertSaved(t, "https://example.com/bar")
}

type mockFetcher struct {
	pages           map[string]string
	responseHeaders map[string]http.Header
	errorURLs       map[string]bool
	fetchDelay      time.Duration
	visited         map[string]time.Time
	hits            []string
	lastHeaders     http.Header
	mu              sync.Mutex
}

func (m *mockFetcher) Fetch(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	m.mu.Lock()
	_, exists := m.pages[url]
	isError := m.errorURLs[url]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("no such page: %s", url)
	}
	if isError {
		return nil, fmt.Errorf("simulated fetch error: %s", url)
	}

	if m.fetchDelay > 0 {
		select {
		case <-time.After(m.fetchDelay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.visited == nil {
		m.visited = make(map[string]time.Time)
	}
	m.visited[url] = time.Now()
	m.hits = append(m.hits, url)
	m.lastHeaders = req.Header
	respHeader := http.Header{}
	if m.responseHeaders != nil {
		if h, ok := m.responseHeaders[url]; ok {
			respHeader = h
		}
	}
	return &http.Response{
		StatusCode: 200,
		Header:     respHeader,
		Body:       io.NopCloser(strings.NewReader(m.pages[url])),
	}, nil
}

func (m *mockFetcher) assertVisited(t *testing.T, url string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.visited[url]; !ok {
		t.Errorf("expected %s to be visited, but it wasn't", url)
	}
}

func (m *mockFetcher) assertNotVisited(t *testing.T, url string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if ts, ok := m.visited[url]; ok {
		t.Errorf("expected %s to not be visited, but it was at %s", url, ts)
	}
}

func (m *mockFetcher) assertTotalVisits(t *testing.T, want int) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if got := len(m.visited); got != want {
		t.Errorf("expected %d visits, got %d", want, got)
	}
}

func (m *mockFetcher) assertHitCount(t *testing.T, url string, want int) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	got := 0
	for _, h := range m.hits {
		if h == url {
			got++
		}
	}
	if got != want {
		t.Errorf("%s: expected %d hits, got %d", url, want, got)
	}
}

func (m *mockFetcher) assertVisitGap(t *testing.T, url1, url2 string, min time.Duration) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	gap := m.visited[url2].Sub(m.visited[url1])
	if gap < 0 {
		gap = -gap
	}
	if gap < min {
		t.Errorf("gap between %s and %s was %v, want >= %v", url1, url2, gap, min)
	}
}

func (m *mockFetcher) assertVisitedBefore(t *testing.T, url1, url2 string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	t1, ok1 := m.visited[url1]
	t2, ok2 := m.visited[url2]
	if !ok1 || !ok2 {
		t.Fatalf("one of %s or %s was not visited", url1, url2)
	}
	if !t1.Before(t2) {
		t.Errorf("expected %s to be visited before %s", url1, url2)
	}
}

// mockStore

type mockStore struct {
	mu    sync.Mutex
	saved []Page
}

func (m *mockStore) SavePage(_ context.Context, p Page) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved = append(m.saved, p)
	return nil
}

func (m *mockStore) assertSaved(t *testing.T, url string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.saved {
		if p.URL == url {
			return
		}
	}
	t.Errorf("expected page %s to be saved, but it wasn't", url)
}

func (m *mockStore) assertNotSaved(t *testing.T, url string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.saved {
		if p.URL == url {
			t.Errorf("expected page %s to not be saved, but it was", url)
			return
		}
	}
}

// helpers

func htmlBody(title string, links ...string) string {
	var sb strings.Builder
	sb.WriteString("<html><body>")
	sb.WriteString("<title>" + title + "</title>")
	for _, l := range links {
		sb.WriteString(`<a href="` + l + `">link</a>`)
	}
	sb.WriteString("</body></html>")
	return sb.String()
}

func robotsTxt(userAgent, disallow, allow, sitemap string) string {
	return fmt.Sprintf("User-agent: %s\nDisallow: %s\nAllow: %s\nSitemap: %s", userAgent, disallow, allow, sitemap)
}

var sampleXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>http://www.example.net/?id=who</loc>
    <lastmod>2009-09-22</lastmod>
  </url>
  <url>
    <loc>http://www.example.net/?id=what</loc>
    <lastmod>2009-09-22</lastmod>
  </url>
  <url>
    <loc>http://www.example.net/?id=how</loc>
    <lastmod>2009-09-22</lastmod>
  </url>
</urlset>
`
