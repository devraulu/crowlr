from __future__ import annotations

import json
import logging

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse, StreamingResponse
from fastapi.templating import Jinja2Templates
from psycopg_pool import ConnectionPool

from crowlr_py import db, retrieval
from crowlr_py.config import Config
from crowlr_py.llm import factory
from crowlr_py.prompting import build_prompt

logger = logging.getLogger("crowlr_py.web")

SEARCH_LIMIT = 500
RETRIEVAL_K = 5


def build_router(cfg: Config, pool: ConnectionPool, templates: Jinja2Templates) -> APIRouter:
    router = APIRouter()

    @router.get("/", response_class=HTMLResponse)
    def index(request: Request):
        return templates.TemplateResponse(request, "index.html", {})

    @router.get("/search", response_class=HTMLResponse)
    def search(request: Request):
        query = request.query_params.get("q", "")
        if not query:
            return templates.TemplateResponse(request, "results.html", {"query": "", "results": [], "count": 0})

        with pool.connection() as conn:
            response = db.search_fulltext(conn, query, SEARCH_LIMIT)

        return templates.TemplateResponse(
            request,
            "results.html",
            {"query": query, "results": response.results, "count": response.total_count},
        )

    @router.get("/summary/stream")
    def summary_stream(request: Request):
        query = request.query_params.get("q", "")

        def event_stream():
            try:
                embeddings = factory.get_embeddings(cfg)
                query_embedding = factory.embed_query_text(cfg, embeddings, query)

                with pool.connection() as conn:
                    chunks = retrieval.search(conn, query_embedding, RETRIEVAL_K)

                urls = [c.page_url for c in chunks]
                yield f"event: sources\ndata: {json.dumps(urls)}\n\n"

                if not chunks:
                    yield "data: I don't have any indexed content related to that question.\n\n"
                    yield "event: done\ndata: \n\n"
                    return

                prompt = build_prompt(query, chunks)
                chat_model = factory.get_chat_model(cfg)
                for chunk in chat_model.stream(prompt):
                    if chunk.content:
                        yield f"data: {chunk.content.replace(chr(10), chr(92) + 'n')}\n\n"

                yield "event: done\ndata: \n\n"
            except Exception:
                logger.exception("summary stream failed", extra={"query": query})
                yield "event: summary-error\ndata: summary unavailable\n\n"

        return StreamingResponse(
            event_stream(),
            media_type="text/event-stream",
            headers={"Cache-Control": "no-cache", "Connection": "keep-alive"},
        )

    return router
