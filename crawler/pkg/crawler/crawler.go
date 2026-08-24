package crawler

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	nurl "net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/benjaminestes/robots"
	"github.com/markusmobius/go-trafilatura"
	"golang.org/x/net/html"
)

type Fetcher interface {
	Fetch(ctx context.Context, url string) (*http.Response, error)
}

type HTTPFetcher struct {
	client *http.Client
}

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return &HTTPFetcher{client: client}
}

func (f HTTPFetcher) Fetch(ctx context.Context, url string) (*http.Response, error) {
	parsedURL, err := nurl.ParseRequestURI(url)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-OK response status", slog.Int("status", resp.StatusCode))
	}

	// ct := resp.Header.Get("Content-Type")
	// if ct == "" || !strings.Contains(strings.ToLower(ct), "text/html") {
	// 	return nil, fmt.Errorf("skipping non-HTML response")
	// }

	return resp, nil
}

type Page struct {
	URL        string
	RawURL     string
	Referrer   string
	StatusCode int
	Outlinks   []string
	FetchedAt  time.Time
	Title      string
	Content    string
	Metadata   Metadata
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

type outcome struct {
	outlinks  []Link
	processed bool
}

func (c *Crawler) Run(ctx context.Context, seeds []Link) error {
	if len(seeds) == 0 {
		return errors.New("no seeds provided")
	}

	c.loadSeeds(seeds)
	slog.Info(
		"crawler starting",
		slog.Int("workers", c.workers),
		slog.Int("seeds", len(seeds)),
		slog.Int("crawl_limit", c.crawlLimit),
		slog.Duration("crawl_delay", c.crawlDelay),
		slog.String("user_agent", c.userAgent),
	)

	jobs := make(chan Link, c.workers)
	out := make(chan outcome, c.workers)

	for range c.workers {
		go c.worker(ctx, jobs, out)
	}

	return c.coordinator(ctx, jobs, out)
}

func (c *Crawler) StatsSnapshot() StatsSnapshot {
	return c.stats.Snapshot(c.frontier.Len())
}

func (c *Crawler) coordinator(ctx context.Context, jobs chan<- Link, out <-chan outcome) error {
	var retryTimer <-chan time.Time
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
			link := c.frontier.PeekEligible(c.politeness)
			if link != nil && c.robotsCheck(ctx, link) {
				jobCh = jobs
				next = *link
			} else {
				if link != nil {
					c.frontier.Remove(*link)
				}
				retryTimer = time.After(c.crawlDelay)
			}
		}

		select {
		case <-ctx.Done():
			close(jobs)
			return ctx.Err()

		case jobCh <- next:
			c.frontier.Remove(next)
			c.hosts[next.Host] = time.Now()
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

func (c *Crawler) worker(ctx context.Context, jobs <-chan Link, out chan<- outcome) {
	for link := range jobs {
		func(link Link) {
			fetchCtx := ctx
			if c.fetchTimeout > 0 {
				var cancel context.CancelFunc
				fetchCtx, cancel = context.WithTimeout(ctx, c.fetchTimeout)
				defer cancel()
			}

			result, err := c.visit(fetchCtx, link)
			if err != nil {
				c.stats.RecordError()
				slog.Error("visit failed", slog.String("url", link.Normalized), slog.Any("err", err))
				out <- outcome{}
				return
			}

			if strings.TrimSpace(result.Content) == "" {
				c.stats.RecordVisit(link.Host, result.StatusCode, result.Duration, 0)
				out <- outcome{}
				return
			}

			c.stats.RecordVisit(link.Host, result.StatusCode, result.Duration, len(result.Outlinks))
			slog.Info("visited", slog.String("url", link.Normalized), slog.Int("status", result.StatusCode), slog.Int("outlinks", len(result.Outlinks)))
			outstrs := make([]string, len(result.Outlinks))
			for i, ol := range result.Outlinks {
				outstrs[i] = ol.Normalized
			}

			var referrer string
			if link.Referrer != nil {
				referrer = link.Referrer.Normalized
			}
			if err := c.store.SavePage(ctx, Page{
				URL:        link.Normalized,
				RawURL:     link.Original,
				Referrer:   referrer,
				StatusCode: result.StatusCode,
				Outlinks:   outstrs,
				FetchedAt:  time.Now(),
				Content:    result.Content,
			}); err != nil {
				slog.Error("failed to save page", slog.String("url", link.Normalized), slog.Any("err", err))
				os.Exit(1)
			}
			out <- outcome{outlinks: result.Outlinks, processed: true}
		}(link)
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
		slog.Debug("robots.txt cache hit", slog.String("url", robotsURL))
		r = cached
	} else {
		r = c.fetchRobots(ctx, robotsURL)
		c.robotsCache[robotsURL] = r
		if r != nil {
			slog.Debug("fetched robots.txt", slog.String("url", robotsURL))
			// c.sitemaps(ctx, r)
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

	resp, err := c.fetcher.Fetch(ctx, robotsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	// defer resp.Body.Close()

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
	if len(r.Sitemaps()) < 1 {
		return
	}
	for _, sitemapURL := range r.Sitemaps()[:1] {
		c.fetchSitemap(ctx, sitemapURL)
	}
}

func (c *Crawler) fetchSitemap(ctx context.Context, sitemapURL string) {
	slog.Debug("fetching sitemap", slog.String("url", sitemapURL))
	link, err := NewLink(sitemapURL)
	if err != nil || !c.robotsCheck(ctx, &link) {
		return
	}

	resp, err := c.fetcher.Fetch(ctx, sitemapURL)
	if err != nil {
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

type Visit struct {
	StatusCode int
	Content    string
	Outlinks   []Link
	Duration   time.Duration
	Metadata   Metadata
}

func (c *Crawler) visit(ctx context.Context, link Link) (Visit, error) {
	slog.Debug("visiting link", slog.String("url", link.Normalized))
	start := time.Now()

	resp, err := c.fetcher.Fetch(ctx, link.Normalized)
	if err != nil {
		slog.Error("fetch failed", slog.Any("error", err))
		return Visit{Duration: time.Since(start)}, err
	}
	slog.Debug("got response without error", slog.String("status", resp.Status))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	result, err := extract(bytes.NewReader(body), link)
	if err != nil {
		return Visit{Duration: time.Since(start)}, err
	}

	return Visit{
		Outlinks: result.links,
		Content:  result.content,
		Metadata: result.metadata,
		Duration: time.Since(start),
	}, nil
}

type Metadata struct {
	Title       string
	Author      string
	URL         string
	Hostname    string
	Description string
	Sitename    string
	Date        time.Time
	Categories  []string
	Tags        []string
	ID          string
	Fingerprint string
	License     string
	Language    string
	Image       string
	PageType    string
}

func (m *Metadata) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("title", m.Title),
		slog.String("URL", m.URL),
	)
}

func convertMetadata(tm trafilatura.Metadata) Metadata {
	return Metadata{
		Title:       tm.Title,
		Author:      tm.Author,
		URL:         tm.URL,
		Hostname:    tm.Hostname,
		Description: tm.Description,
		Sitename:    tm.Sitename,
		Date:        tm.Date,
		Categories:  tm.Categories,
		Tags:        tm.Tags,
		ID:          tm.ID,
		Fingerprint: tm.Fingerprint,
		License:     tm.License,
		Language:    tm.Language,
		Image:       tm.Image,
		PageType:    tm.PageType,
	}
}

type Extract struct {
	metadata Metadata
	links    []Link
	content  string
}

func extract(r io.Reader, referrer Link) (*Extract, error) {
	parsedURL, err := nurl.ParseRequestURI(referrer.Normalized)
	if err != nil {
		return nil, err
	}

	opts := trafilatura.Options{
		EnableFallback:  true,
		ExcludeComments: true,
		IncludeImages:   false,
		IncludeLinks:    true,
		OriginalURL:     parsedURL,
	}
	result, err := trafilatura.Extract(r, opts)
	if err != nil {
		return nil, err
	}

	slog.Debug("trafilatura result", slog.Any("metadata", result.Metadata))

	return &Extract{
		content:  result.ContentText,
		links:    outlinks(result.ContentNode, referrer),
		metadata: convertMetadata(result.Metadata),
	}, nil
}

func outlinks(node *html.Node, referrer Link) (links []Link) {
	base, err := nurl.ParseRequestURI(referrer.Normalized)
	if err != nil {
		slog.Debug("error parsing referrer", slog.Any("error", err))
	}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link, err := NewLink(resolve(attr.Val, base), WithReferrer(&referrer))
					if err != nil {
						slog.Debug("error creating outlink", slog.Any("err", err))
					}
					links = append(links, link)
					break
				} else if attr.Key == "base" && attr.Val != "" {
					newReferrer, err := NewLink(attr.Val, WithReferrer(referrer.Referrer))
					if err != nil {
						break
					}
					newBase, err := nurl.ParseRequestURI(newReferrer.Normalized)
					if err != nil {
						break
					}
					referrer = newReferrer
					base = newBase
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}

	visit(node)
	return links
	// z := html.NewTokenizer(r)

	// var sb strings.Builder

	// skipDepth := 0
	// inTitle := false

	// blockTags := map[string]bool{
	// 	"p": true, "div": true, "br": true, "li": true, "ul": true, "ol": true,
	// 	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	// 	"section": true, "article": true, "header": true, "footer": true,
	// }

	// whitespace := func(s string) {
	// 	s = strings.TrimSpace(s)
	// 	if s == "" {
	// 		return
	// 	}
	// 	if sb.Len() > 0 && !strings.HasSuffix(sb.String(), " ") {
	// 		sb.WriteByte(' ')
	// 	}
	// 	sb.WriteString(s)
	// }

	// for {
	// 	tt := z.Next()
	// 	switch tt {
	// 	case html.ErrorToken:
	// 		if z.Err() == io.EOF {
	// 			return &ExtractionResult{
	// 				links: links, content: strings.TrimSpace(sb.String()),
	// 			}, nil
	// 		}
	//
	// 		return links, strings.TrimSpace(sb.String()), z.Err()
	//
	// 	case html.TextToken:
	// 		t := string(z.Text())
	// 		if skipDepth > 0 {
	// 			continue
	// 		}
	// 		if inTitle {
	// 			title = strings.TrimSpace(t)
	// 			continue
	// 		}
	// 		whitespace(t)
	//
	// 	case html.StartTagToken, html.SelfClosingTagToken:
	// 		name, hasAttr := z.TagName()
	// 		tag := strings.ToLower(string(name))
	//
	// 		if tag == "script" || tag == "style" || tag == "noscript" {
	// 			skipDepth++
	// 			if tt == html.SelfClosingTagToken {
	// 				skipDepth--
	// 			}
	// 			continue
	// 		}
	//
	// 		if tag == "title" {
	// 			inTitle = true
	// 		}
	//
	// 		if tt == html.SelfClosingTagToken && tag == "br" {
	// 			sb.WriteByte('\n')
	// 		}
	//
	// 		// Handle attrs (base + links)
	// 		if hasAttr {
	// 			for {
	// 				k, v, more := z.TagAttr()
	// 				kn := strings.ToLower(string(k))
	// 				val := string(v)
	//
	// 				if tag == "base" && kn == "href" {
	// 					if newBase, err := base.Parse(val); err == nil {
	// 						base = newBase
	// 					}
	// 				}
	// 				if tag == "a" && kn == "href" {
	// 					if nl, err := NewLink(resolve(val, base), WithReferrer(referrer.Normalized)); err == nil {
	// 						links = append(links, nl)
	// 					}
	// 				}
	//
	// 				if !more {
	// 					break
	// 				}
	// 			}
	// 		}
	//
	// 		// Insert whitespace for block tags so sentences don't glue together
	// 		if blockTags[tag] && (tt == html.StartTagToken) {
	// 			// p/div/etc. usually start a new block
	// 			if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
	// 				sb.WriteByte('\n')
	// 			}
	// 		}
	//
	// 	case html.EndTagToken:
	// 		name, _ := z.TagName()
	// 		tag := strings.ToLower(string(name))
	// 		if tag == "title" {
	// 			inTitle = false
	// 		}
	// 		if tag == "script" || tag == "style" || tag == "noscript" {
	// 			if skipDepth > 0 {
	// 				skipDepth--
	// 			}
	// 		}
	// 	}
	//
	// }
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
