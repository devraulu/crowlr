package crawler

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/benjaminestes/robots"
	"golang.org/x/net/html"
)

type Fetcher interface {
	Fetch(*http.Request) (*http.Response, error)
}

type HTTPFetcher struct {
	client *http.Client
}

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return &HTTPFetcher{client: client}
}

func (f *HTTPFetcher) Fetch(req *http.Request) (*http.Response, error) {
	return f.client.Do(req)
}

func (c *Crawler) fetch(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return c.fetcher.Fetch(req)
}

type Page struct {
	URL        string
	RawURL     string
	Referrer   string
	StatusCode int
	HTML       string
	Outlinks   []string
	FetchedAt  time.Time
	Title      string
}

type Store interface {
	SavePage(ctx context.Context, p Page) error
}

type NullStore struct{}

func (NullStore) SavePage(_ context.Context, _ Page) error { return nil }

type CrawlerOption func(*Crawler)

func WithCrawlDelay(d time.Duration) CrawlerOption {
	return func(c *Crawler) { c.crawlDelay = d }
}

func WithUserAgent(ua string) CrawlerOption {
	return func(c *Crawler) { c.userAgent = ua }
}

func WithWorkers(n int) CrawlerOption {
	return func(c *Crawler) { c.workers = n }
}

func WithCrawlLimit(n int) CrawlerOption {
	return func(c *Crawler) { c.crawlLimit = n }
}

func WithFetchTimeout(d time.Duration) CrawlerOption {
	return func(c *Crawler) { c.fetchTimeout = d }
}

func NewCrawler(fetcher Fetcher, store Store, opts ...CrawlerOption) *Crawler {
	crawler := &Crawler{
		fetcher:     fetcher,
		store:       store,
		frontier:    NewFrontier(),
		workers:     runtime.NumCPU(),
		hosts:       make(map[string]time.Time),
		robotsCache: map[string]*robots.Robots{},
		stats:       NewStats(),
	}

	for _, opt := range opts {
		opt(crawler)
	}
	return crawler
}

type Crawler struct {
	fetcher      Fetcher
	frontier     *Frontier
	workers      int
	crawlLimit   int
	crawlDelay   time.Duration
	fetchTimeout time.Duration
	robotsCache  map[string]*robots.Robots
	hosts        map[string]time.Time
	userAgent    string
	store        Store
	stats        *Stats
}

func (c *Crawler) Run(ctx context.Context, seeds []Link) error {
	if len(seeds) == 0 {
		return errors.New("no seeds provided")
	}

	c.loadSeeds(seeds)
	slog.Info("crawler starting",
		slog.Int("workers", c.workers),
		slog.Int("seeds", len(seeds)),
		slog.Int("crawl_limit", c.crawlLimit),
		slog.Duration("crawl_delay", c.crawlDelay),
		slog.String("user_agent", c.userAgent),
	)

	type outcome struct {
		outlinks  []Link
		processed bool
	}

	jobs := make(chan Link, c.workers)
	out := make(chan outcome, c.workers)
	var retryTimer <-chan time.Time

	for range c.workers {
		go func() {
			for link := range jobs {
				fetchCtx := ctx
				if c.fetchTimeout > 0 {
					var cancel context.CancelFunc
					fetchCtx, cancel = context.WithTimeout(ctx, c.fetchTimeout)
					defer cancel()
				}
				result := c.visit(fetchCtx, link)
				if result.Err != nil {
					c.stats.RecordError()
					slog.Error("visit failed", slog.String("url", link.Normalized), slog.Any("err", result.Err))
					out <- outcome{}
					continue
				}

				if result.Body == nil {
					c.stats.RecordVisit(link.Host, result.StatusCode, result.Duration, result.LastModified, 0)
					out <- outcome{}
					continue
				}

				c.stats.RecordVisit(link.Host, result.StatusCode, result.Duration, result.LastModified, len(result.Outlinks))
				slog.Info("visited", slog.String("url", link.Normalized), slog.Int("status", result.StatusCode), slog.Int("outlinks", len(result.Outlinks)))
				outstrs := make([]string, len(result.Outlinks))
				for i, ol := range result.Outlinks {
					outstrs[i] = ol.Normalized
				}

				if err := c.store.SavePage(ctx, Page{
					URL:        link.Normalized,
					RawURL:     link.Original,
					Referrer:   link.Referrer,
					StatusCode: result.StatusCode,
					HTML:       string(result.Body),
					Outlinks:   outstrs,
					FetchedAt:  time.Now(),
					Title:      result.Title,
				}); err != nil {
					slog.Warn("failed to save page", slog.String("url", link.Normalized), slog.Any("err", err))
				}
				out <- outcome{outlinks: result.Outlinks, processed: true}
			}
		}()
	}

	var pending, pagesProcessed int
	for {
		limitReached := c.crawlLimit > 0 && pagesProcessed >= c.crawlLimit

		if (c.frontier.Len() == 0 || limitReached) && pending == 0 {
			if limitReached {
				slog.Info("crawl limit reached", slog.Int("limit", c.crawlLimit))
			} else {
				slog.Info("crawl complete")
			}
			c.stats.Log(c.frontier.Len())
			close(jobs)
			return nil
		}

		var (
			jobCh chan<- Link
			next  Link
		)

		if c.frontier.Len() > 0 && !limitReached {
			link := c.frontier.PopEligible(c.politeness)
			if link != nil && c.robotsCheck(ctx, link) {
				c.hosts[link.Host] = time.Now()
				jobCh = jobs
				next = *link
			} else {
				retryTimer = time.After(c.crawlDelay)
			}
		}

		select {
		case <-ctx.Done():
			close(jobs)
			return ctx.Err()

		case jobCh <- next:
			pending++

		case <-retryTimer:
			continue

		case o := <-out:
			pending--
			if o.processed {
				pagesProcessed++
			}
			for _, ol := range o.outlinks {
				if !c.frontier.Push(ol) {
					slog.Debug("frontier duplicate, skipping", slog.String("url", ol.Normalized))
					continue
				}
			}
		}
	}
}

func (c *Crawler) politeness(link Link) bool {
	if t, ok := c.hosts[link.Host]; ok {
		if time.Since(t) < c.crawlDelay {
			return false
		}
	}

	return true
}

func (c *Crawler) loadSeeds(seeds []Link) {
	for _, link := range seeds {
		c.frontier.Push(link)
	}
}

func (c *Crawler) robotsCheck(ctx context.Context, link *Link) bool {
	robotsURL, err := robots.Locate(link.Normalized)
	if err != nil {
		// shouldn't fail because NewLink already fails if URL is invalid
		return true
	}

	var r *robots.Robots
	if cached, ok := c.robotsCache[robotsURL]; ok {
		r = cached
	} else {
		r = c.fetchRobots(ctx, robotsURL)
		c.robotsCache[robotsURL] = r
		if r != nil {
			slog.Debug("fetched robots.txt", slog.String("url", robotsURL))
			c.sitemaps(ctx, r)
		}
	}

	if r != nil && !r.Test(c.userAgent, link.Normalized) {
		slog.Debug("disallowed by robots.txt", slog.String("url", link.Normalized))
		return false
	}

	return true
}

func (c *Crawler) fetchRobots(ctx context.Context, robotsURL string) (result *robots.Robots) {
	defer func() {
		if recover() != nil {
			result = nil
		}
	}()

	resp, err := c.fetch(ctx, robotsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return nil
	}

	r, err := robots.From(resp.StatusCode, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	return r
}

func (c *Crawler) sitemaps(ctx context.Context, r *robots.Robots) {
	for _, sitemapURL := range r.Sitemaps() {
		c.fetchSitemap(ctx, sitemapURL)
	}
}

func (c *Crawler) fetchSitemap(ctx context.Context, sitemapURL string) {
	slog.Debug("fetching sitemap", slog.String("url", sitemapURL))
	link, err := NewLink(sitemapURL)
	if err != nil || !c.robotsCheck(ctx, &link) {
		return
	}

	resp, err := c.fetch(ctx, sitemapURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var index struct {
		Sitemaps []struct {
			Loc string `xml:"loc"`
		} `xml:"sitemap"`
	}
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		for _, s := range index.Sitemaps {
			c.fetchSitemap(ctx, s.Loc)
		}
		return
	}

	var urlset struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(body, &urlset); err != nil {
		return
	}
	for _, u := range urlset.URLs {
		if l, err := NewLink(u.Loc); err == nil {
			c.frontier.Push(l)
			slog.Debug("sitemap url queued", slog.String("url", u.Loc))
		}
	}
}

type VisitResult struct {
	StatusCode   int
	Body         []byte
	Outlinks     []Link
	Title        string
	Duration     time.Duration
	LastModified *time.Time
	Err          error
}

func (c *Crawler) visit(ctx context.Context, link Link) VisitResult {
	start := time.Now()
	resp, err := c.fetch(ctx, link.Normalized)
	dur := time.Since(start)
	if err != nil {
		return VisitResult{Err: err, Duration: dur}
	}
	defer resp.Body.Close()

	var lastMod *time.Time
	if raw := resp.Header.Get("Last-Modified"); raw != "" {
		if t, err := http.ParseTime(raw); err == nil {
			lastMod = &t
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "text/html") {
		slog.Debug("skipping non-HTML response", slog.String("url", link.Normalized), slog.String("content_type", ct))
		return VisitResult{StatusCode: resp.StatusCode, Duration: dur, LastModified: lastMod}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VisitResult{Err: err, Duration: dur}
	}

	title, outlinks, err := extract(bytes.NewReader(body), link)
	if err != nil {
		return VisitResult{Err: err, Duration: dur}
	}

	return VisitResult{
		StatusCode:   resp.StatusCode,
		Body:         body,
		Outlinks:     outlinks,
		Title:        title,
		Duration:     dur,
		LastModified: lastMod,
	}
}

func extract(r io.Reader, referrer Link) (string, []Link, error) {
	base, _ := url.Parse(referrer.Normalized)
	z := html.NewTokenizer(r)
	var links []Link
	var title string
	var inTitle bool

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return title, links, nil
			}
			return title, links, z.Err()

		case html.TextToken:
			if inTitle {
				title = string(z.Text())
				inTitle = false
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			if string(name) == "title" {
				inTitle = false
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)

			if tag == "title" {
				inTitle = true
			}

			if !hasAttr {
				continue
			}

			for {
				k, v, more := z.TagAttr()
				if tag == "base" && string(k) == "href" {
					if newBase, err := base.Parse(string(v)); err == nil {
						base = newBase
					}
				}
				if tag == "a" && string(k) == "href" {
					nl, err := NewLink(resolve(string(v), base), WithReferrer(referrer.Normalized))
					if err == nil {
						links = append(links, nl)
					}
				}
				if !more {
					break
				}
			}
		}
	}
}

func resolve(ref string, base *url.URL) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	abs := base.ResolveReference(u)

	scheme := strings.ToLower(abs.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}

	return abs.String()
}
