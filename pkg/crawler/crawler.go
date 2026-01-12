package crawler

import (
	"context"
	"log/slog"
	"time"

	"github.com/benjaminestes/robots"
	frontier "github.com/devraulu/crowlr/pkg"
	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/process"
	"github.com/devraulu/crowlr/pkg/storage"
)

type CrawlStats struct {
	StartTime      time.Time
	PagesProcessed int
	PagesErrored   int
	PagesSkipped   int
}

func (s *CrawlStats) Elapsed() time.Duration {
	return time.Since(s.StartTime)
}

func (s *CrawlStats) PagesPerSecond() float64 {
	elapsed := s.Elapsed().Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(s.PagesProcessed) / elapsed
}

type Crawler struct {
	cfg         *config.Config
	frontier    *frontier.Frontier
	store       storage.Storage
	robotsCache map[string]*robots.Robots
	Stats       CrawlStats
}

type Job struct {
	referrer string
	rawUrl   string
	url      string
}

func New(cfg *config.Config, f *frontier.Frontier, s storage.Storage) *Crawler {
	return &Crawler{
		cfg:         cfg,
		frontier:    f,
		store:       s,
		robotsCache: make(map[string]*robots.Robots),
	}
}

func (c *Crawler) Start(ctx context.Context) {
	c.Stats.StartTime = time.Now()

	jobs := make(chan frontier.Candidate, c.cfg.Crawler.Workers)
	results := make(chan CrawlResult, c.cfg.Crawler.Workers)

	for i := 0; i < c.cfg.Crawler.Workers; i++ {
		go c.worker(ctx, i, jobs, results)
	}

	c.coordinator(ctx, jobs, results)

	slog.Info("crawl complete",
		slog.Int("processed", c.Stats.PagesProcessed),
		slog.Int("errored", c.Stats.PagesErrored),
		slog.Int("skipped", c.Stats.PagesSkipped),
		slog.Duration("elapsed", c.Stats.Elapsed()),
		slog.Float64("pages_per_sec", c.Stats.PagesPerSecond()),
	)
}

func (c *Crawler) coordinator(ctx context.Context, jobs chan<- frontier.Candidate, results <-chan CrawlResult) {
	activeWorkers := 0
	for {
		if c.cfg.Crawler.CrawlLimit > 0 && c.Stats.PagesProcessed >= c.cfg.Crawler.CrawlLimit {
			for activeWorkers > 0 {
				select {
				case res := <-results:
					activeWorkers--
					c.processResult(ctx, res)
				case <-ctx.Done():
					return
				}
			}
			return
		}

		var candidate *frontier.Candidate
		var jobsChan chan<- frontier.Candidate
		var waitTime time.Duration

		if c.frontier.Len() > 0 {
			candidate, waitTime = c.frontier.Pop(c.cfg.Politeness.GetDelay())
			if candidate == nil {
				slog.Info("frontier empty and no active workers. mission complete.")
				return
			}
			if candidate.Normalized != "" {
				r := process.CheckRobots(candidate.Normalized, c.robotsCache)
				if r != nil && !r.Test(c.cfg.Crawler.UserAgent, candidate.Normalized) {
					slog.Info("robots.txt disallowed", slog.Any("candidate", candidate))
					continue
				}

				jobsChan = jobs
			} else if waitTime > 0 {
				time.Sleep(waitTime)
				continue
			}
		} else if activeWorkers == 0 {
			slog.Info("frontier empty and no active workers. mission complete.")
			return
		}

		if candidate != nil {
			select {
			case jobsChan <- *candidate:
				activeWorkers++
				slog.Info("job dispatched", slog.Any("candidate", candidate), slog.Int("active_workers", activeWorkers), slog.Int("seen", c.frontier.Len()))

			case res := <-results:
				activeWorkers--
				c.processResult(ctx, res)

			case <-ctx.Done():
				return
			}
		} else {
			select {
			case res := <-results:
				activeWorkers--
				c.processResult(ctx, res)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *Crawler) processResult(ctx context.Context, res CrawlResult) {
	if res.Error != nil {
		c.Stats.PagesErrored++
		slog.Error("crawl failed", slog.String("url", res.URL), slog.Any("err", res.Error))
		return
	}

	if res.PageData == nil {
		c.Stats.PagesSkipped++
		return
	}

	c.Stats.PagesProcessed++
	slog.Info("crawl success",
		slog.String("url", res.URL),
		slog.Int("outlinks", len(res.Outlinks)),
		slog.Int("processed", c.Stats.PagesProcessed),
		slog.Float64("pages_per_sec", c.Stats.PagesPerSecond()),
	)

	if err := c.store.SavePage(ctx, *res.PageData); err != nil {
		c.Stats.PagesErrored++
		slog.Error("failed to save page", slog.String("url", res.URL), slog.Any("err", err))
	}

	for _, link := range res.Outlinks {
		if c.cfg.Crawler.CrawlLimit > 0 && c.Stats.PagesProcessed >= c.cfg.Crawler.CrawlLimit {
			slog.Info("crawl limit reached, stopping outlink push",
				slog.Int("processed", c.Stats.PagesProcessed),
				slog.Int("queued", c.frontier.Len()),
				slog.Int("limit", c.cfg.Crawler.CrawlLimit),
			)
			break
		}
		c.frontier.Push(link.Normalized, link.Original, res.URL)
	}
}
