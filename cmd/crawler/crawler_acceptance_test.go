package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/devraulu/crowlr/pkg/crawler"
)

func TestCrawlerStoresPagesEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/page-a">Page A</a>
			<a href="/page-b">Page B</a>
		</body></html>`)
	})
	mux.HandleFunc("/page-a", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Page A</h1></body></html>`)
	})
	mux.HandleFunc("/page-b", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Page B</h1></body></html>`)
	})

	app := newTestApp(t, mux)

	seed, err := crawler.NewLink(app.server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	c := crawler.NewCrawler(
		crawler.NewHTTPFetcher(&http.Client{}),
		app.store,
		crawler.WithWorkers(1),
	)

	if err := c.Run(t.Context(), []crawler.Link{seed}); err != nil {
		t.Fatal(err)
	}

	type row struct {
		url        string
		statusCode int
		outlinks   []string
	}

	rows, err := app.db.QueryContext(t.Context(),
		`SELECT url, status_code, outlinks FROM pages ORDER BY url`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var pages []row
	for rows.Next() {
		var p row
		var rawOutlinks []byte
		if err := rows.Scan(&p.url, &p.statusCode, &rawOutlinks); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(rawOutlinks, &p.outlinks)
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	base := app.server.URL

	want := []row{
		{url: base + "/", statusCode: 200, outlinks: []string{base + "/page-a", base + "/page-b"}},
		{url: base + "/page-a", statusCode: 200, outlinks: []string{}},
		{url: base + "/page-b", statusCode: 200, outlinks: []string{}},
	}

	if len(pages) != len(want) {
		t.Fatalf("expected %d pages, got %d: %v", len(want), len(pages), pages)
	}

	for i, w := range want {
		got := pages[i]
		if got.url != w.url {
			t.Errorf("[%d] url: got %q, want %q", i, got.url, w.url)
		}
		if got.statusCode != w.statusCode {
			t.Errorf("[%d] status_code: got %d, want %d", i, got.statusCode, w.statusCode)
		}
		if len(got.outlinks) != len(w.outlinks) {
			t.Errorf("[%d] outlinks: got %v, want %v", i, got.outlinks, w.outlinks)
		}
	}
}
