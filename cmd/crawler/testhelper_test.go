package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/devraulu/crowlr/pkg/crawler"
)

var pgContainer *postgres.PostgresContainer
var logSink testWriter

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(&logSink, nil)))

	ctx := context.Background()

	var err error
	pgContainer, err = postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("crowlr"),
		postgres.WithPassword("password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	code := m.Run()

	if err := testcontainers.TerminateContainer(pgContainer); err != nil {
		log.Printf("terminate postgres container: %v", err)
	}

	os.Exit(code)
}

type testApp struct {
	store  *crawler.PostgresStore
	db     *sql.DB
	server *httptest.Server
}

func newTestApp(t *testing.T, handler http.Handler) *testApp {
	t.Helper()
	ctx := context.Background()

	baseConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	adminDB, err := sql.Open("postgres", baseConnStr)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	defer adminDB.Close()

	dbName := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	u, _ := url.Parse(baseConnStr)
	u.Path = "/" + dbName
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := crawler.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &testApp{
		store:  crawler.NewPostgresStore(db),
		db:     db,
		server: server,
	}
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

func setLogger(t testing.TB) {
	t.Helper()
	logSink.mu.Lock()
	logSink.t = t
	logSink.mu.Unlock()
	t.Cleanup(func() {
		logSink.mu.Lock()
		logSink.t = nil
		logSink.mu.Unlock()
	})
}
