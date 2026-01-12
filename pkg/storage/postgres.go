package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) SavePage(ctx context.Context, p Page) error {
	jsonOutlinks, err := json.Marshal(p.Outlinks)
	if err != nil {
		return err
	}

	var id int
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO pages (url, normalized_url, timestamp, title, content, html, status_code, outlinks, last_modified, referrer)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		p.RawURL, p.URL, p.Timestamp, p.Title, p.Content, p.HTML, p.StatusCode, jsonOutlinks, p.LastModified, p.Referrer,
	).Scan(&id)

	if err != nil {
		return err
	}

	slog.Info("saved page", "id", id)
	return nil
}

func (s *PostgresStorage) SaveSitemap(ctx context.Context, sm Sitemap) error {
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO sitemaps (url, last_checked, status_code, content)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (url) DO UPDATE
		SET last_checked = EXCLUDED.last_checked, status_code = EXCLUDED.status_code, content = EXCLUDED.content
		RETURNING id`,
		sm.URL, sm.LastChecked, sm.StatusCode, sm.Content,
	).Scan(&id)

	if err != nil {
		return err
	}

	slog.Info("saved sitemap", "id", id)
	return nil
}

func (s *PostgresStorage) Search(ctx context.Context, query string, limit int) (SearchResponse, error) {
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
			ts_headline('english', COALESCE(content, ''), query, 'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=25') AS snippet,
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

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
