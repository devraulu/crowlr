package process

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/benjaminestes/robots"
)

func CheckRobots(url string, cache map[string]*robots.Robots) *robots.Robots {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic in robots.txt parsing, assuming allowed", slog.String("url", url), slog.Any("panic", r))
		}
	}()

	robotsURL, err := robots.Locate(url)
	if err != nil {
		return nil
	}

	if r, ok := cache[robotsURL]; ok {
		return r
	}

	r, err := getRobots(robotsURL)
	if err != nil {
		slog.Warn("failed to fetch robots.txt", slog.String("url", robotsURL), slog.Any("err", err))
		cache[robotsURL] = nil
		return nil
	}

	cache[robotsURL] = r
	return r
}

func getRobots(url string) (*robots.Robots, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	slog.Debug("robots.txt response",
		slog.String("url", url),
		slog.Int("status_code", resp.StatusCode),
		slog.Int("body_length", len(body)),
		slog.String("body_preview", string(body[:min(len(body), 200)])),
	)

	return robots.From(resp.StatusCode, bytes.NewReader(body))
}
