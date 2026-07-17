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

func TestExtract(t *testing.T) {
	t.Run("strips script tags", func(t *testing.T) {
		input := sampleHTML
		_, _, got, err := extract(strings.NewReader(input), Link{})
		if err != nil {
			t.Errorf("expected no err but got %s", err)
		}

		if strings.Contains(got, "function") {
			t.Errorf("expected no function in body but got: %s", got)
		}
		if strings.Contains(got, "<script>") {
			t.Errorf("expected no script tag in body but got: %s", got)
		}
		if strings.Contains(got, "style") {
			t.Errorf("expected no style tag in body but got: %s", got)
		}
	})
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
var sampleHTML = `<!doctypehtml><html lang="en"><title>Lorem Ipsum - All the facts - Lipsum generator</title><meta content="Lorem Ipsum, Lipsum, Lorem, Ipsum, Text, Generate, Generator, Facts, Information, What, Why, Where, Dummy Text, Typesetting, Printing, de Finibus, Bonorum et Malorum, de Finibus Bonorum et Malorum, Extremes of Good and Evil, Cicero, Latin, Garbled, Scrambled, Lorem ipsum dolor sit amet, dolor, sit amet, consectetur, adipiscing, elit, sed, eiusmod, tempor, incididunt"name="keywords"><meta content="Reference site about Lorem Ipsum, giving information on its origins, as well as a random Lipsum generator."name="description"><meta content="width=device-width,initial-scale=1"name="viewport"><meta content="text/html; charset=utf-8"http-equiv="content-type"><script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-9385669889547030"crossorigin="anonymous"></script><script async src="https://www.googletagmanager.com/gtag/js?id=G-W02QY0T0GX"></script><script>function gtag(){dataLayer.push(arguments)}window.dataLayer=window.dataLayer||[],gtag("js",new Date),gtag("config","G-W02QY0T0GX")</script><link href="https://a.pub.network/"rel="preconnect"crossorigin><link href="https://b.pub.network/"rel="preconnect"crossorigin><link href="https://c.pub.network/"rel="preconnect"crossorigin><link href="https://d.pub.network/"rel="preconnect"crossorigin><link href="https://c.amazon-adsystem.com"rel="preconnect"crossorigin><link href="https://s.amazon-adsystem.com"rel="preconnect"crossorigin><link href="https://btloader.com/"rel="preconnect"crossorigin><link href="https://api.btloader.com/"rel="preconnect"crossorigin><link href="https://cdn.confiant-integrations.net"rel="preconnect"crossorigin><link href="https://a.pub.network/lipsum-com/cls.css"rel="stylesheet"><script data-cfasync="false">var freestar=freestar||{};freestar.queue=freestar.queue||[],freestar.config=freestar.config||{},freestar.config.enabled_slots=[],freestar.initCallback=function(){0===freestar.config.enabled_slots.length?freestar.initCallbackCalled=!1:freestar.newAdSlots(freestar.config.enabled_slots)}</script><script data-cfasync="false"async src="https://a.pub.network/lipsum-com/pubfig.min.js"></script><link href="/favicon.ico"rel="icon"type="image/x-icon"><link href="/css/230223.css"rel="stylesheet"><div id="Outer"><div class="banner"style="min-height:90px"><div id="lipsumcom_header"align="center"data-freestar-ad="__336x280 __970x250"><script data-cfasync="false">freestar.config.enabled_slots.push({placementName:"lipsumcom_header",slotId:"lipsumcom_header"})</script></div></div><div id="Inner"><div id="Languages"><a href="http://hy.lipsum.com/">Հայերեն</a> <a href="http://sq.lipsum.com/">Shqip</a> <span class="ltr"dir="ltr"><a href="http://ar.lipsum.com/">‫العربية</a></span> <a href="http://bg.lipsum.com/">Български</a> <a href="http://ca.lipsum.com/">Català</a> <a href="http://cn.lipsum.com/">中文简体</a> <a href="http://hr.lipsum.com/">Hrvatski</a> <a href="http://cs.lipsum.com/">Česky</a> <a href="http://da.lipsum.com/">Dansk</a> <a href="http://nl.lipsum.com/">Nederlands</a> <a href="http://www.lipsum.com/"class="zz">English</a> <a href="http://et.lipsum.com/">Eesti</a> <a href="http://ph.lipsum.com/">Filipino</a> <a href="http://fi.lipsum.com/">Suomi</a> <a href="http://fr.lipsum.com/">Français</a> <a href="http://ka.lipsum.com/">ქართული</a> <a href="http://de.lipsum.com/">Deutsch</a> <a href="http://el.lipsum.com/">Ελληνικά</a> <span class="ltr"dir="ltr"><a href="http://he.lipsum.com/">‫עברית</a></span> <a href="http://hi.lipsum.com/">हिन्दी</a> <a href="http://hu.lipsum.com/">Magyar</a> <a href="http://id.lipsum.com/">Indonesia</a> <a href="http://it.lipsum.com/">Italiano</a> <a href="http://lv.lipsum.com/">Latviski</a> <a href="http://lt.lipsum.com/">Lietuviškai</a> <a href="http://mk.lipsum.com/">македонски</a> <a href="http://ms.lipsum.com/">Melayu</a> <a href="http://no.lipsum.com/">Norsk</a> <a href="http://pl.lipsum.com/">Polski</a> <a href="http://pt.lipsum.com/">Português</a> <a href="http://ro.lipsum.com/">Româna</a> <a href="http://ru.lipsum.com/">Pyccкий</a> <a href="http://sr.lipsum.com/">Српски</a> <a href="http://sk.lipsum.com/">Slovenčina</a> <a href="http://sl.lipsum.com/">Slovenščina</a> <a href="http://es.lipsum.com/">Español</a> <a href="http://sv.lipsum.com/">Svenska</a> <a href="http://th.lipsum.com/">ไทย</a> <a href="http://tr.lipsum.com/">Türkçe</a> <a href="http://uk.lipsum.com/">Українська</a> <a href="http://vi.lipsum.com/">Tiếng Việt</a></div><h1>Lorem Ipsum</h1><h4>"Neque porro quisquam est qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit..."</h4><h5>"There is no one who loves pain itself, who seeks after it and wants to have it, simply because it is pain..."</h5><hr><div id="Content"><div id="bannerL"><div id="lipsumcom_left_siderail"align="center"data-freestar-ad="__300x600"><script data-cfasync="false">freestar.config.enabled_slots.push({placementName:"lipsumcom_left_siderail",slotId:"lipsumcom_left_siderail"})</script></div></div><div id="bannerR"><div id="lipsumcom_right_siderail"align="center"data-freestar-ad="__300x600"><script data-cfasync="false">freestar.config.enabled_slots.push({placementName:"lipsumcom_right_siderail",slotId:"lipsumcom_right_siderail"})</script></div></div><div id="Panes"><div><h2>What is Lorem Ipsum?</h2><p><strong>Lorem Ipsum</strong> is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since 1966, when designers at Letraset and James Mosley, the librarian at St Bride Printing Library in London, took a 1914 Cicero translation and scrambled it to make dummy text for Letraset's Body Type sheets. It has survived not only many decades, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised thanks to these sheets and more recently with desktop publishing software like Aldus PageMaker and Microsoft Word including versions of Lorem Ipsum.</div><div><h2>Why do we use it?</h2><p>It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using 'Content here, content here', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for 'lorem ipsum' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</div><br><div><h2>Where does it come from?</h2><p>Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.<p>The standard chunk of Lorem Ipsum used since 1966 is reproduced below for those interested. Sections 1.10.32 and 1.10.33 from "de Finibus Bonorum et Malorum" by Cicero are also reproduced in their exact original form, accompanied by English versions from the 1914 translation by H. Rackham.</div><div><h2>Where can I get some?</h2><p>There are many variations of passages of Lorem Ipsum available, but the majority have suffered alteration in some form, by injected humour, or randomised words which don't look even slightly believable. If you are going to use a passage of Lorem Ipsum, you need to be sure there isn't anything embarrassing hidden in the middle of text. All the Lorem Ipsum generators on the Internet tend to repeat predefined chunks as necessary, making this the first true generator on the Internet. It uses a dictionary of over 200 Latin words, combined with a handful of model sentence structures, to generate Lorem Ipsum which looks reasonable. The generated Lorem Ipsum is therefore always free from repetition, injected humour, or non-characteristic words etc.<form action="/feed/html"method="post"><table style="width:100%"><tr><td rowspan="2"><input id="amount"name="amount"value="5"size="3"><td rowspan="2"><table style="text-align:left"><tr><td style="width:20px"><input id="paras"name="what"value="paras"type="radio"checked><td><label for="paras">paragraphs</label><tr><td style="width:20px"><input id="words"name="what"value="words"type="radio"><td><label for="words">words</label><tr><td style="width:20px"><input id="bytes"name="what"value="bytes"type="radio"><td><label for="bytes">bytes</label><tr><td style="width:20px"><input id="lists"name="what"value="lists"type="radio"><td><label for="lists">lists</label></table><td style="width:20px"><input id="start"name="start"value="yes"type="checkbox"checked><td style="text-align:left"><label for="start">Start with 'Lorem<br>ipsum dolor sit amet...'</label><tr><td><td style="text-align:left"><input id="generate"name="generate"value="Generate Lorem Ipsum"type="submit"></table></form></div><br></div><hr><div class="boxed"><strong>Donate:</strong> If you use this site regularly and would like to help keep the site on the Internet, please consider donating a small sum to help pay for the hosting and bandwidth bill. There is no minimum donation, any sum is appreciated - click <a href="/donate"class="lnk"target="_blank">here</a> to donate using PayPal. Thank you for your support. Donate bitcoin: 16UQLq1HZ3CNwhvgrarV6pMoA2CDjb4tyF</div><hr><div class="boxed"><strong>Translations:</strong> Can you help translate this site into a foreign language ? Please email us with details if you can help.</div><hr><div class="boxed">There is a set of mock banners available <a href="/banners"class="lnk">here</a> in three colours and in a range of standard banner sizes:<br><a href="/banners"><img alt="Banners"height="60"src="/images/banners/black_234x60.gif"width="234"></a><a href="/banners"><img alt="Banners"height="60"src="/images/banners/grey_234x60.gif"width="234"></a><a href="/banners"><img alt="Banners"height="60"src="/images/banners/white_234x60.gif"width="234"></a></div><hr><div id="Packages"class="boxed"><a href="https://github.com/traviskaufman/node-lipsum"rel="noopener"rel="nofollow"target="_blank">NodeJS</a> <a href="https://code.google.com/p/pypsum/"rel="noopener"rel="nofollow"target="_blank">Python Interface</a> <a href="https://gtklipsum.sourceforge.net/"rel="noopener"rel="nofollow"target="_blank">GTK Lipsum</a> <a href="https://github.com/gsavage/lorem_ipsum/tree/master"rel="noopener"rel="nofollow"target="_blank">Rails</a> <a href="https://github.com/cerkit/LoremIpsum/"rel="noopener"rel="nofollow"target="_blank">.NET</a></div><hr><div style="margin:10px 0"><div id="lipsumcom_incontent"align="center"data-freestar-ad="__336x280 __728x90"><script data-cfasync="false">freestar.config.enabled_slots.push({placementName:"lipsumcom_incontent",slotId:"lipsumcom_incontent"})</script></div></div><hr><div id="Translation"><h3>The standard Lorem Ipsum passage, used since 1966</h3><p>"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."<h3>Section 1.10.32 of "de Finibus Bonorum et Malorum", written by Cicero in 45 BC</h3><p>"Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt. Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit, sed quia non numquam eius modi tempora incidunt ut labore et dolore magnam aliquam quaerat voluptatem. Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam, nisi ut aliquid ex ea commodi consequatur? Quis autem vel eum iure reprehenderit qui in ea voluptate velit esse quam nihil molestiae consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla pariatur?"<h3>1914 translation by H. Rackham</h3><p>"But I must explain to you how all this mistaken idea of denouncing pleasure and praising pain was born and I will give you a complete account of the system, and expound the actual teachings of the great explorer of the truth, the master-builder of human happiness. No one rejects, dislikes, or avoids pleasure itself, because it is pleasure, but because those who do not know how to pursue pleasure rationally encounter consequences that are extremely painful. Nor again is there anyone who loves or pursues or desires to obtain pain of itself, because it is pain, but because occasionally circumstances occur in which toil and pain can procure him some great pleasure. To take a trivial example, which of us ever undertakes laborious physical exercise, except to obtain some advantage from it? But who has any right to find fault with a man who chooses to enjoy a pleasure that has no annoying consequences, or one who avoids a pain that produces no resultant pleasure?"<h3>Section 1.10.33 of "de Finibus Bonorum et Malorum", written by Cicero in 45 BC</h3><p>"At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium voluptatum deleniti atque corrupti quos dolores et quas molestias excepturi sint occaecati cupiditate non provident, similique sunt in culpa qui officia deserunt mollitia animi, id est laborum et dolorum fuga. Et harum quidem rerum facilis est et expedita distinctio. Nam libero tempore, cum soluta nobis est eligendi optio cumque nihil impedit quo minus id quod maxime placeat facere possimus, omnis voluptas assumenda est, omnis dolor repellendus. Temporibus autem quibusdam et aut officiis debitis aut rerum necessitatibus saepe eveniet ut et voluptates repudiandae sint et molestiae non recusandae. Itaque earum rerum hic tenetur a sapiente delectus, ut aut reiciendis voluptatibus maiores alias consequatur aut perferendis doloribus asperiores repellat."<h3>1914 translation by H. Rackham</h3><p>"On the other hand, we denounce with righteous indignation and dislike men who are so beguiled and demoralized by the charms of pleasure of the moment, so blinded by desire, that they cannot foresee the pain and trouble that are bound to ensue; and equal blame belongs to those who fail in their duty through weakness of will, which is the same as saying through shrinking from toil and pain. These cases are perfectly simple and easy to distinguish. In a free hour, when our power of choice is untrammelled and when nothing prevents our being able to do what we like best, every pleasure is to be welcomed and every pain avoided. But in certain circumstances and owing to the claims of duty or the obligations of business it will frequently occur that pleasures have to be repudiated and annoyances accepted. The wise man therefore always holds in these matters to this principle of selection: he rejects pleasures to secure other greater pleasures, or else he endures pains to avoid worse pains."</div></div><hr><div class="boxed"><a href="mailto:help@lipsum.com"style="text-decoration:none">help@lipsum.com</a><br><a href="/privacy"rel="nofollow"style="text-decoration:none">Privacy Policy · <button id="pmLink">Privacy Manager</button></div></div><div class="banner"style="margin-bottom:110px"><div id="lipsumcom_leaderboard_bottom"align="center"data-freestar-ad="__336x280 __970x250"><script data-cfasync="false">freestar.config.enabled_slots.push({placementName:"lipsumcom_leaderboard_bottom",slotId:"lipsumcom_leaderboard_bottom"})</script></div></div></div><style>#pmLink{visibility:hidden;font-family:"Open Sans",Arial,sans-serif;font-size:14px;color:#000;text-decoration:none;cursor:pointer;background:0 0;border:none}#pmLink:hover{visibility:visible;color:#d00}</style>`
