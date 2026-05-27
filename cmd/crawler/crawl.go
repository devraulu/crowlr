package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/logger"
	"github.com/devraulu/crowlr/pkg/crawler"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "Start crawling from seed URLs",
	RunE:  runCrawl,
}

var (
	flagSeeds        string
	flagWorkers      int
	flagDelay        string
	flagFetchTimeout string
	flagLimit        int
	flagUserAgent    string
	flagLogLevel     string
	flagLogFormat    string
)

func init() {
	crawlCmd.Flags().StringVarP(&flagSeeds, "seeds", "s", "", "seeds file (overrides config)")
	crawlCmd.Flags().IntVarP(&flagWorkers, "workers", "w", 0, "number of workers (overrides config)")
	crawlCmd.Flags().StringVarP(&flagDelay, "delay", "d", "", "politeness delay, e.g. 500ms (overrides config)")
	crawlCmd.Flags().StringVar(&flagFetchTimeout, "fetch-timeout", "", "fetch timeout, e.g. 10s (overrides config)")
	crawlCmd.Flags().IntVarP(&flagLimit, "limit", "l", -1, "max pages to crawl, 0 = unlimited (overrides config)")
	crawlCmd.Flags().StringVar(&flagUserAgent, "user-agent", "", "user agent string (overrides config)")
	crawlCmd.Flags().StringVar(&flagLogLevel, "log-level", "", "log level: debug, info, warn, error (overrides config)")
	crawlCmd.Flags().StringVar(&flagLogFormat, "log-format", "", "log format: text, json (overrides config)")
	rootCmd.AddCommand(crawlCmd)
}

func runCrawl(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cmd.Flags().Changed("log-level") {
		cfg.Logging.Level = flagLogLevel
	}
	if cmd.Flags().Changed("log-format") {
		cfg.Logging.Format = flagLogFormat
	}
	logger.InitLogger(cfg)

	if cmd.Flags().Changed("seeds") {
		cfg.Crawler.SeedsFile = flagSeeds
	}
	if cmd.Flags().Changed("workers") {
		cfg.Crawler.Workers = flagWorkers
	}
	if cmd.Flags().Changed("delay") {
		cfg.Politeness.Delay = flagDelay
	}
	if cmd.Flags().Changed("fetch-timeout") {
		cfg.Politeness.FetchTimeout = flagFetchTimeout
	}
	if cmd.Flags().Changed("limit") {
		cfg.Crawler.CrawlLimit = flagLimit
	}
	if cmd.Flags().Changed("user-agent") {
		cfg.Crawler.UserAgent = flagUserAgent
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	seeds, err := crawler.LoadSeedsFile(cfg.Crawler.SeedsFile)
	if err != nil {
		return fmt.Errorf("failed to load seeds: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	slog.Debug("database connected")

	slog.Debug("running database migrations")
	if err := crawler.RunMigrations(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	slog.Debug("database migrations complete")

	store := crawler.NewPostgresStore(db)

	client := &http.Client{}
	c := crawler.NewCrawler(crawler.NewHTTPFetcher(client),
		store,
		crawler.WithUserAgent(cfg.Crawler.UserAgent),
		crawler.WithCrawlDelay(cfg.Politeness.GetDelay()),
		crawler.WithWorkers(cfg.Crawler.Workers),
		crawler.WithCrawlLimit(cfg.Crawler.CrawlLimit),
		crawler.WithFetchTimeout(cfg.Politeness.GetFetchTimeout()),
	)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		select {
		case s := <-sig:
			slog.Info("received signal, shutting down", slog.String("signal", s.String()))
			stop()
		case <-ctx.Done():
		}
	}()

	if err := c.Run(ctx, seeds); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
