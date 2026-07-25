package backfill

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/devraulu/crowlr/pkg/backfill/chunker"
	"github.com/devraulu/crowlr/pkg/backfill/embed"
	"github.com/pgvector/pgvector-go"
)

const (
	batchSize    = 16
	maxWords     = 400
	overlapWords = 50
)

func Backfill(ctx context.Context, db *sql.DB, ollamaURL string) error {
	rows, err := db.QueryContext(ctx, `
	SELECT id, html FROM pages
	WHERE id NOT IN (SELECT DISTINCT page_id FROM chunks)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	slog.Info("backfill starting")

	var pagesProcessed, chunksInserted int

	for rows.Next() {
		var pageID int64
		var content string
		if err := rows.Scan(&pageID, &content); err != nil {
			return err
		}

		pageChunks := chunker.ChunkText(content, maxWords, overlapWords)
		slog.Debug("chunked page", slog.Int64("page_id", pageID), slog.Int("chunks", len(pageChunks)))

		for i := 0; i < len(pageChunks); i += batchSize {
			batch := pageChunks[i:min(i+batchSize, len(pageChunks))]

			texts := make([]string, len(batch))
			for j, c := range batch {
				texts[j] = c.Content
			}

			vectors, err := embed.Batch(ollamaURL, "nomic-embed-text", embed.WithDocPrefix(texts))
			if err != nil {
				slog.Error("embed batch failed", slog.Int64("page_id", pageID), slog.Any("err", err))
				return fmt.Errorf("page: %d: %w", pageID, err)
			}
			for j, c := range batch {
				var lastInsertId int
				err := db.QueryRowContext(ctx, `
				INSERT INTO chunks (page_id, chunk_index, content, embedding)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (page_id, chunk_index) DO NOTHING RETURNING id`, pageID, c.Index, c.Content, pgvector.NewVector(vectors[j])).Scan(&lastInsertId)
				if err != nil {
					slog.Error("insert chunk failed", slog.Int64("page_id", pageID), slog.Int("chunk_index", c.Index), slog.Any("err", err))
					return fmt.Errorf("insert page=%d chunk=%d: %w", pageID, c.Index, err)
				}
				chunksInserted++
			}
		}

		pagesProcessed++
		slog.Info("page backfilled", slog.Int64("page_id", pageID), slog.Int("chunks", len(pageChunks)))
	}

	if err := rows.Err(); err != nil {
		return err
	}

	slog.Info("backfill complete", slog.Int("pages", pagesProcessed), slog.Int("chunks", chunksInserted))
	return nil
}
