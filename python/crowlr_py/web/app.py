from __future__ import annotations

import json
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from markupsafe import Markup
from pgvector.psycopg import register_vector
from psycopg_pool import ConnectionPool

from crowlr_py.config import Config, load
from crowlr_py.web.routes import build_router

WEB_DIR = Path(__file__).resolve().parent
TEMPLATES_DIR = WEB_DIR / "templates"
STATIC_DIR = WEB_DIR / "static"


def create_app(cfg: Config | None = None) -> FastAPI:
    cfg = cfg or load()

    pool = ConnectionPool(cfg.dsn, open=False, configure=register_vector)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        pool.open()
        yield
        pool.close()

    app = FastAPI(lifespan=lifespan)
    app.state.db_pool = pool

    templates = Jinja2Templates(directory=str(TEMPLATES_DIR))
    templates.env.filters["tojson"] = lambda v: Markup(json.dumps(v))

    app.include_router(build_router(cfg, pool, templates))
    app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")

    return app


app = create_app()
