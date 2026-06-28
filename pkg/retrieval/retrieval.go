package retrieval

import (
	"context"
	"database/sql"

	"github.com/pgvector/pgvector-go"
)

type RetrievedChunk struct {
	Content  string
	PageURL  string
	Distance float64
}

func Search(ctx context.Context, db *sql.DB, queryEmbedding []float32, k int) ([]RetrievedChunk, error) {

	rows, err := db.QueryContext(ctx, `
		SELECT c.content, p.url, c.embedding <=> $1 AS distance
		FROM chunks c
		JOIN pages p ON p.id = c.page_id
		ORDER BY c.embedding <=> $1
		LIMIT $2
		`, pgvector.NewVector(queryEmbedding), k)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []RetrievedChunk

	for rows.Next() {
		var rc RetrievedChunk
		if err := rows.Scan(&rc.Content, &rc.PageURL, &rc.Distance); err != nil {
			return nil, err
		}
		results = append(results, rc)
	}

	return results, rows.Err()
}
