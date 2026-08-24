CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS chunks (
    id SERIAL PRIMARY KEY,
    page_id SERIAL NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    UNIQUE (page_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS chunks_page_id_idx ON chunks (page_id);
CREATE INDEX IF NOT EXISTS chunks_metadata_gin_idx ON chunks USING GIN (metadata);
CREATE INDEX IF NOT EXISTS chunks_tsv_gin_idx ON chunks USING GIN (tsv);

CREATE INDEX chunks_embedding_hnsw_idx ON chunks USING hnsw (embedding vector_cosine_ops);
