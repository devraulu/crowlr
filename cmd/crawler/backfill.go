package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/devraulu/crowlr/pkg/backfill"
	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/logger"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Create embeddings for RAG and backfill",
	RunE:  runBackfill,
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}

func runBackfill(cmd *cobra.Command, _ []string) error {
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

	if err := backfill.Backfill(ctx, db, cfg.LLM.OllamaURL); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
