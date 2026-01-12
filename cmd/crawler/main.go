package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	_ "github.com/lib/pq"

	frontier "github.com/devraulu/crowlr/pkg"
	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/logger"
	"github.com/devraulu/crowlr/pkg/storage"
)

func main() {
	var err error

	cfg, err := config.Load("config.toml")
	if err != nil {
		slog.Error("fatal: couldn't load config", slog.Any("err", err))
		os.Exit(1)
	}

	logger.InitLogger(cfg)

	f := frontier.NewFrontier()

	if err := frontier.LoadSeeds(cfg.Crawler.SeedsFile, f); err != nil {
		slog.Error("fatal: couldn't load seeds", slog.Any("err", err))
		os.Exit(1)
	}

	pool, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		slog.Error("fatal: couldn't open database", slog.Any("err", err))
		os.Exit(1)
	}

	defer pool.Close()

	if err := storage.RunMigrations(pool); err != nil {
		slog.Error("fatal: failed to run migrations", "err", err)
		os.Exit(1)
	}

	store := storage.NewPostgresStorage(pool)

	c := crawler.New(cfg, f, store)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	var wg sync.WaitGroup

	appSignal := make(chan os.Signal, 1)
	signal.Notify(appSignal, syscall.SIGINT, syscall.SIGQUIT)

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(ctx)
		stop()
	}()

	select {
	case s := <-appSignal:
		slog.Info("received system signal", slog.String("signal", s.String()))
		stop()
	case <-ctx.Done():
		slog.Info("context done, stopping")
	}

	wg.Wait()
	slog.Info("shutdown complete")
}
