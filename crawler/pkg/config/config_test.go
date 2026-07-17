package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadParsesPolitenessDurations(t *testing.T) {
	path := writeConfig(t, `
dsn = "postgres://example"

[politeness]
delay = "250ms"
fetch_timeout = "3s"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := cfg.Politeness.GetDelay(); got != 250*time.Millisecond {
		t.Fatalf("delay = %v, want 250ms", got)
	}
	if got := cfg.Politeness.GetFetchTimeout(); got != 3*time.Second {
		t.Fatalf("fetch timeout = %v, want 3s", got)
	}
}

func TestLoadRejectsInvalidPolitenessDuration(t *testing.T) {
	path := writeConfig(t, `
dsn = "postgres://example"

[politeness]
delay = "ten seconds"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), "politeness.delay") {
		t.Fatalf("error = %q, want field name", err.Error())
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
