package main

import (
	"context"
	"database/sql"
	"embed"
	"html/template"
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

		slog.Info("search", "query", query)

		results, err := store.Search(context.Background(), query, 50)
		if err != nil {
			slog.Error("search failed", "query", query, "err", err)
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}

		slog.Info("search complete", "query", query, "results", len(results))
		tmpl.ExecuteTemplate(w, "results.html", results)
	}
}
