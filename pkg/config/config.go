package config

import (
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	DSN        string           `toml:"dsn"`
	Crawler    CrawlerConfig    `toml:"crawler"`
	Politeness PolitenessConfig `toml:"politeness"`
	Logging    LoggingConfig    `toml:"logging"`
	LLM        LLMConfig        `toml:"llm"`
}

type CrawlerConfig struct {
	UserAgent  string `toml:"user_agent"`
	SeedsFile  string `toml:"seeds_file"`
	CrawlLimit int    `toml:"crawl_limit"`
	Workers    int    `toml:"workers"`
}

type PolitenessConfig struct {
	Delay        string `toml:"delay"`
	FetchTimeout string `toml:"fetch_timeout"`

	delay        time.Duration
	fetchTimeout time.Duration
}

type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type LLMConfig struct {
	OllamaURL  string `toml:"ollama_url"`
	EmbedModel string `toml:"embed_model"`
	GenModel   string `toml:"gen_model"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	cfg.Crawler.SeedsFile = "seeds.txt"
	cfg.Politeness.Delay = "1s"
	cfg.Politeness.FetchTimeout = "10s"
	cfg.Logging.Format = "text"
	cfg.Logging.Level = "info"
	cfg.LLM.EmbedModel = "nomic-embed-text"
	cfg.LLM.GenModel = "llama3.2"

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if err := c.Politeness.Validate(); err != nil {
		return err
	}
	return nil
}

func (c *PolitenessConfig) Validate() error {
	delay, err := parseDuration("politeness.delay", c.Delay)
	if err != nil {
		return err
	}

	fetchTimeout, err := parseDuration("politeness.fetch_timeout", c.FetchTimeout)
	if err != nil {
		return err
	}

	c.delay = delay
	c.fetchTimeout = fetchTimeout
	return nil
}

func (c *PolitenessConfig) GetDelay() time.Duration {
	return c.delay
}

func (c *PolitenessConfig) GetFetchTimeout() time.Duration {
	return c.fetchTimeout
}

func parseDuration(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return d, nil
}
