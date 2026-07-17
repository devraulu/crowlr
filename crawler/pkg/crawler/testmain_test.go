package crawler

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
)

var logSink testWriter

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(&logSink, nil)))
	os.Exit(m.Run())
}

type testWriter struct {
	mu sync.Mutex
	t  testing.TB
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.t != nil {
		w.t.Log(strings.TrimRight(string(p), "\n"))
	}
	return len(p), nil
}

