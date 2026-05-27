package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/devraulu/crowlr/pkg/config"
)

func InitLogger(cfg *config.Config) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Logging.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				// Only use bunyan levels if JSON
				if cfg.Logging.Format != "text" {
					level := a.Value.Any().(slog.Level)
					return slog.Int(a.Key, bunyanLevel(level))
				}
			}
			return a
		},
	}

	if cfg.Logging.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With(
		"name", "crowlr",
		"pid", os.Getpid(),
		"hostname", hostname,
	)
	slog.SetDefault(logger)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func bunyanLevel(level slog.Level) int {
	switch {
	case level >= slog.LevelError:
		return 50
	case level >= slog.LevelWarn:
		return 40
	case level >= slog.LevelInfo:
		return 30
	case level >= slog.LevelDebug:
		return 20
	default:
		return 10
	}
}
