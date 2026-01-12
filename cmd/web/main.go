package main

import (
	"context"
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"log/slog"
	"net/http"

	_ "github.com/lib/pq"

	"github.com/devraulu/crowlr/pkg/config"
	"github.com/devraulu/crowlr/pkg/logger"
	"github.com/devraulu/crowlr/pkg/storage"
)

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var staticFiles embed.FS

type SearchResults struct {
	Results []storage.SearchResult
	Count   int
	Query   string
}

var tmpl *template.Template

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatal(err)
	}

	logger.InitLogger(cfg)

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := storage.NewPostgresStorage(db)

	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	}
	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templates, "templates/*.html"))

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/search", handleSearch(store))

	staticFS, _ := fs.Sub(staticFiles, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	addr := ":8080"
	slog.Info("starting web server", "addr", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	slog.Info("request", "method", r.Method, "path", r.URL.Path)
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func handleSearch(store *storage.PostgresStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			tmpl.ExecuteTemplate(w, "results.html", nil)
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
		tmpl.ExecuteTemplate(w, "results.html", searchResults)
	}
}
