package crawler

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"strings"
)

var ErrNoSeeds = errors.New("no seeds loaded")

func LoadSeedsFile(path string) ([]Link, error) {
	slog.Info("loading seeds", slog.String("path", path))
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var links []Link
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		link, err := NewLink(raw)
		if err != nil {
			slog.Warn("invalid seed url, skipping", slog.String("url", raw), slog.Any("err", err))
			continue
		}
		links = append(links, link)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(links) == 0 {
		return nil, ErrNoSeeds
	}

	slog.Info("seeds loaded", slog.Int("count", len(links)))
	return links, nil
}
