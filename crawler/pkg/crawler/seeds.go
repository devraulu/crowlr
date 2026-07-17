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

	var raw []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw = append(raw, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ParseSeeds(raw)
}

func ParseSeeds(rawSeeds []string) ([]Link, error) {
	var links []Link
	for _, line := range rawSeeds {
		raw := strings.TrimSpace(line)
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

	if len(links) == 0 {
		return nil, ErrNoSeeds
	}

	slog.Info("seeds loaded", slog.Int("count", len(links)))
	return links, nil
}
