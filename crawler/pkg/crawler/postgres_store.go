package crawler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
)

type PostgresStore struct {
	db *sql.DB
}

type SearchResult struct {
	URL     string
	Title   string
	Snippet string
	Rank    float64
}

type SearchResponse struct {
	Results    []SearchResult
	TotalCount int
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) SavePage(ctx context.Context, p Page) error {
	outlinks, err := json.Marshal(p.Outlinks)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO pages (url, raw_url, title, referrer, status_code, outlinks, fetched_at, content, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (url) DO UPDATE SET
			url = EXCLUDED.url,
			raw_url = EXCLUDED.raw_url,
			title = EXCLUDED.title,
			referrer = EXCLUDED.referrer,
			status_code = EXCLUDED.status_code,
			content = EXCLUDED.content,
			outlinks = EXCLUDED.outlinks,
			fetched_at = EXCLUDED.fetched_at,
			metadata = EXCLUDED.metadata;
		`,
		p.URL, p.RawURL, p.Title, p.Referrer, p.StatusCode, outlinks, p.FetchedAt, p.Content, metadata,
	)
	return err
}

func (s *PostgresStore) Search(ctx context.Context, query string, limit int) (SearchResponse, error) {
	slog.Debug("search query", "query", query, "limit", limit)

	// Get total count first
	var totalCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pages, websearch_to_tsquery('english', $1) query
		WHERE textsearch @@ query`,
		query,
	).Scan(&totalCount)
	if err != nil {
		slog.Error("search count query failed", "query", query, "err", err)
		return SearchResponse{}, err
	}

	// Get limited results
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			url,
			COALESCE(title, ''),
			ts_headline('english', COALESCE(text, ''), query, 'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=25') AS snippet,
			ts_rank_cd(textsearch, query, 32) AS rank
		FROM pages, websearch_to_tsquery('english', $1) query
		WHERE textsearch @@ query
		ORDER BY rank DESC
		LIMIT $2`,
		query, limit,
	)
	if err != nil {
		slog.Error("search query failed", "query", query, "err", err)
		return SearchResponse{}, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.URL, &r.Title, &r.Snippet, &r.Rank); err != nil {
			slog.Error("search scan failed", "query", query, "err", err)
			return SearchResponse{}, err
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		slog.Error("search rows iteration failed", "query", query, "err", err)
		return SearchResponse{}, err
	}

	slog.Info("search complete", "query", query, "results", len(results), "total", totalCount)
	return SearchResponse{
		Results:    results,
		TotalCount: totalCount,
	}, nil
}
