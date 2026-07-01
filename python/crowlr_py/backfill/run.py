from __future__ import annotations

import logging

from crowlr_py import chunking, config, db
from crowlr_py.llm import factory

MAX_WORDS = 400
OVERLAP_WORDS = 50
BATCH_SIZE = 16

logger = logging.getLogger("crowlr_py.backfill")


def _batched(items: list, size: int):
    for i in range(0, len(items), size):
        yield items[i : i + size]


def main() -> None:
    logging.basicConfig(level=logging.INFO)

    cfg = config.load()
    conn = db.connect(cfg.dsn)
    embeddings = factory.get_embeddings(cfg)

    logger.info("backfill starting")

    pages_processed = 0
    chunks_inserted = 0

    for page_id, html in db.fetch_pages_needing_chunks(conn):
        page_chunks = chunking.chunk_text(html, MAX_WORDS, OVERLAP_WORDS)
        logger.debug("chunked page", extra={"page_id": page_id, "chunks": len(page_chunks)})

        for batch in _batched(page_chunks, BATCH_SIZE):
            texts = [c.content for c in batch]
            vectors = factory.embed_doc_texts(cfg, embeddings, texts)
            for c, vector in zip(batch, vectors):
                inserted_id = db.insert_chunk(conn, page_id, c.index, c.content, vector)
                if inserted_id is not None:
                    chunks_inserted += 1

        pages_processed += 1
        logger.info("page backfilled", extra={"page_id": page_id, "chunks": len(page_chunks)})

    logger.info("backfill complete", extra={"pages": pages_processed, "chunks": chunks_inserted})


if __name__ == "__main__":
    main()
