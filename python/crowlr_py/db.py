from __future__ import annotations

from dataclasses import dataclass
from typing import Iterator

import psycopg
from pgvector.psycopg import register_vector


def connect(dsn: str) -> psycopg.Connection:
    conn = psycopg.connect(dsn, autocommit=True)
    register_vector(conn)
    return conn


@dataclass
class SearchResult:
    url: str
    title: str
    snippet: str
    rank: float


@dataclass
class SearchResponse:
    results: list[SearchResult]
    total_count: int


def search_fulltext(conn: psycopg.Connection, query: str, limit: int) -> SearchResponse:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT COUNT(*)
            FROM pages, websearch_to_tsquery('english', %s) query
            WHERE textsearch @@ query
            """,
            (query,),
        )
        (total_count,) = cur.fetchone()

        cur.execute(
            """
            SELECT
                url,
                COALESCE(title, ''),
                ts_headline('english', COALESCE(html, ''), query, 'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=25') AS snippet,
                ts_rank_cd(textsearch, query, 32) AS rank
            FROM pages, websearch_to_tsquery('english', %s) query
            WHERE textsearch @@ query
            ORDER BY rank DESC
            LIMIT %s
            """,
            (query, limit),
        )
        results = [SearchResult(*row) for row in cur.fetchall()]

    return SearchResponse(results=results, total_count=total_count)


def fetch_pages_needing_chunks(conn: psycopg.Connection) -> Iterator[tuple[int, str]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id, html FROM pages
            WHERE id NOT IN (SELECT DISTINCT page_id FROM chunks)
            """
        )
        yield from cur.fetchall()


def insert_chunk(
    conn: psycopg.Connection,
    page_id: int,
    chunk_index: int,
    content: str,
    embedding: list[float],
) -> int | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO chunks (page_id, chunk_index, content, embedding)
            VALUES (%s, %s, %s, %s)
            ON CONFLICT (page_id, chunk_index) DO NOTHING
            RETURNING id
            """,
            (page_id, chunk_index, content, embedding),
        )
        row = cur.fetchone()
        return row[0] if row else None
