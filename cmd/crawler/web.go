package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/devraulu/crowlr/pkg/answer"
	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/crawler"
	"github.com/devraulu/crowlr/pkg/logger"
	"github.com/devraulu/crowlr/pkg/retrieval"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web search interface",
	RunE:  runWeb,
}

var flagPort int

func init() {
	webCmd.Flags().IntVarP(&flagPort, "port", "p", 8080, "port to listen on")
	rootCmd.AddCommand(webCmd)
}

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var staticFiles embed.FS

type SearchResults struct {
	Results []crawler.SearchResult
	Count   int
	Query   string
}

var tmpl *template.Template

func runWeb(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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

	if err := crawler.RunMigrations(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	store := crawler.NewPostgresStore(db)

	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"inc":      func(i int) int { return i + 1 },
	}
	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templates, "templates/*.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/search", handleSearch(store))
	mux.HandleFunc("/summary/stream", handleSummaryStream(db, cfg.LLM.OllamaURL, cfg.LLM.EmbedModel, cfg.LLM.GenModel))

	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	addr := fmt.Sprintf(":%d", flagPort)
	slog.Info("starting web server", "addr", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	slog.Info("request", "method", r.Method, "path", r.URL.Path)
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		slog.Error("failed to render index", slog.Any("err", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleSearch(store *crawler.PostgresStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			if err := tmpl.ExecuteTemplate(w, "results.html", nil); err != nil {
				slog.Error("failed to render results", slog.Any("err", err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		slog.Info("search", slog.String("query", query))

		searchResponse, err := store.Search(context.Background(), query, 500)
		if err != nil {
			slog.Error("search failed", slog.String("query", query), slog.Any("err", err))
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}

		searchResults := SearchResults{
			Results: searchResponse.Results,
			Count:   searchResponse.TotalCount,
			Query:   query,
		}

		slog.Info("search complete", slog.String("query", query), slog.Int("results", len(searchResponse.Results)), slog.Int("total", searchResponse.TotalCount))
		if err := tmpl.ExecuteTemplate(w, "results.html", searchResults); err != nil {
			slog.Error("failed to render results", slog.Any("err", err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func handleSummaryStream(db *sql.DB, ollamaURL, embedModel, genModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		slog.Info("summary", slog.String("query", query))

		onChunks := func(chunks []retrieval.RetrievedChunk) {
			urls := make([]string, len(chunks))
			for i, c := range chunks {
				urls[i] = c.PageURL
			}
			data, _ := json.Marshal(urls)
			fmt.Fprintf(w, "event: sources\ndata: %s\n\n", data)
			flusher.Flush()
		}

		onToken := func(token string) {
			fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(token, "\n", "\\n"))
			flusher.Flush()
		}

		if err := answer.AnswerStream(r.Context(), db, ollamaURL, embedModel, genModel, query, onChunks, onToken); err != nil {
			slog.Error("summary stream failed", slog.String("query", query), slog.Any("err", err))
			fmt.Fprint(w, "event: summary-error\ndata: summary unavailable\n\n")
			flusher.Flush()
			return
		}

		fmt.Fprint(w, "event: done\ndata: \n\n")
		flusher.Flush()
	}
}
