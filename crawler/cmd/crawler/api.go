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
	"time"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/jobs"
	"github.com/devraulu/crowlr/pkg/logger"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Serve an HTTP API for triggering and monitoring crawls",
	RunE:  runAPI,
}

var (
	flagAPIPort      int
	flagAPILogLevel  string
	flagAPILogFormat string
)

func init() {
	apiCmd.Flags().IntVarP(&flagAPIPort, "port", "p", 8080, "HTTP listen port")
	apiCmd.Flags().StringVar(&flagAPILogLevel, "log-level", "", "log level: debug, info, warn, error (overrides config)")
	apiCmd.Flags().StringVar(&flagAPILogFormat, "log-format", "", "log format: text, json (overrides config)")
	rootCmd.AddCommand(apiCmd)
}

func runAPI(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cmd.Flags().Changed("log-level") {
		cfg.Logging.Level = flagAPILogLevel
	}
	if cmd.Flags().Changed("log-format") {
		cfg.Logging.Format = flagAPILogFormat
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger.InitLogger(cfg)

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
	fetcher := crawler.NewHTTPFetcher(&http.Client{})
	records := jobs.NewPostgresRecordStore(db)

	mgr := jobs.NewManager(fetcher, store, records, jobs.Defaults{
		UserAgent:    cfg.Crawler.UserAgent,
		CrawlDelay:   cfg.Politeness.GetDelay(),
		Workers:      cfg.Crawler.Workers,
		CrawlLimit:   cfg.Crawler.CrawlLimit,
		FetchTimeout: cfg.Politeness.GetFetchTimeout(),
	})

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

	h := &apiHandler{mgr: mgr, cfg: cfg, ctx: ctx}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.createCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.getCrawl)
	mux.HandleFunc("GET /crawls", h.listCrawls)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", flagAPIPort), Handler: mux}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("api server listening", slog.Int("port", flagAPIPort))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
