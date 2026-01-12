package config

import (
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	DSN        string           `toml:"dsn"`
	Crawler    CrawlerConfig    `toml:"crawler"`
	Politeness PolitenessConfig `toml:"politeness"`
	Logging    LoggingConfig    `toml:"logging"`
}

type CrawlerConfig struct {
	UserAgent  string `toml:"user_agent"`
	SeedsFile  string `toml:"seeds_file"`
	CrawlLimit int    `toml:"crawl_limit"`
	Workers    int    `toml:"workers"`
}

type PolitenessConfig struct {
	Delay         string `toml:"delay"`
	RobotsTimeout string `toml:"robots_timeout"`
}

type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	cfg.Crawler.SeedsFile = "seeds.txt"
	cfg.Politeness.Delay = "1s"
	cfg.Logging.Format = "text"
	cfg.Logging.Level = "info"

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *PolitenessConfig) GetDelay() time.Duration {
	d, err := time.ParseDuration(c.Delay)
	if err != nil {
		return 1 * time.Second // Fallback
	}
	return d
}
