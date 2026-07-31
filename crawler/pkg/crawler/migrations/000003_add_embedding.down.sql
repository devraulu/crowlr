DROP INDEX IF EXISTS chunks_embedding_idx;
DROP INDEX IF EXISTS chunks_page_id_idx;
DROP INDEX IF EXISTS chunks_tsv_gin_idx;
DROP INDEX IS EXISTS chunks_embedding_hnsw_idx;

DROP TABLE IF EXISTS chunks;

DROP EXTENSION IF EXISTS vector;
