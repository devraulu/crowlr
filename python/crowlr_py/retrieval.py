from __future__ import annotations

from dataclasses import dataclass

import psycopg


@dataclass
class RetrievedChunk:
    content: str
    page_url: str
    distance: float


def search(conn: psycopg.Connection, query_embedding: list[float], k: int) -> list[RetrievedChunk]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT c.content, p.url, c.embedding <=> %s::vector AS distance
            FROM chunks c
            JOIN pages p ON p.id = c.page_id
            ORDER BY c.embedding <=> %s::vector
            LIMIT %s
            """,
            (query_embedding, query_embedding, k),
        )
        return [RetrievedChunk(*row) for row in cur.fetchall()]
