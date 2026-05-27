package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/logger"
	"github.com/devraulu/crowlr/pkg/crawler"
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

	store := crawler.NewPostgresStore(db)

	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	}
	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templates, "templates/*.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/search", handleSearch(store))

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
