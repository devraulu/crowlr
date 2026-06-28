package answer

import (
	"context"
	"database/sql"

	"github.com/devraulu/crowlr/pkg/embed"
	"github.com/devraulu/crowlr/pkg/generation"
	"github.com/devraulu/crowlr/pkg/retrieval"
)

func Answer(ctx context.Context, db *sql.DB, ollamaURL, embedModel, genModel, query string) (string, []retrieval.RetrievedChunk, error) {
	qEmbedding, err := embed.Query(ollamaURL, embedModel, query)
	if err != nil {
		return "", nil, err
	}

	chunks, err := retrieval.Search(ctx, db, qEmbedding, 5)
	if err != nil {
		return "", nil, err
	}

	if len(chunks) == 0 {
		return "I don't have any indexed content related to that question.", nil, nil
	}

	prompt := generation.BuildPrompt(query, chunks)

	answer, err := generation.Generate(ollamaURL, genModel, prompt)
	if err != nil {
		return "", nil, err
	}

	return answer, chunks, nil
}

func AnswerStream(ctx context.Context, db *sql.DB, ollamaURL, embedModel, genModel, query string, onChunks func([]retrieval.RetrievedChunk), onToken func(string)) error {
	qEmbedding, err := embed.Query(ollamaURL, embedModel, query)
	if err != nil {
		return err
	}

	chunks, err := retrieval.Search(ctx, db, qEmbedding, 5)
	if err != nil {
		return err
	}

	onChunks(chunks)

	if len(chunks) == 0 {
		onToken("I don't have any indexed content related to that question.")
		return nil
	}

	prompt := generation.BuildPrompt(query, chunks)

	_, err = generation.GenerateStream(ctx, ollamaURL, genModel, prompt, onToken)
	return err
}
