CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE chunks (
    id SERIAL PRIMARY KEY,
    page_id SERIAL NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    UNIQUE (page_id, chunk_index)
);

CREATE INDEX embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);
