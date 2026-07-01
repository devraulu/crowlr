# crowlr_py

Search UI, backfill (chunking + embedding), and LLM/RAG calling for crowlr. Reads/writes the same PostgreSQL `pages`/`chunks` tables that the Go crawler provisions — this project does not run its own migrations. Run `../tmp/crawler crawl` (or apply `../pkg/crawler/migrations/*.sql` manually) at least once before using this.

Assumes a locally running [Ollama](https://ollama.com) (`ollama serve`) with `nomic-embed-text` and `llama3.2:3b` pulled — same requirement the old Go `backfill`/`web` commands had.

## Setup

```bash
# install uv: https://docs.astral.sh/uv/getting-started/installation/
uv sync
```

Config is read from `../config.toml` (shared with the Go crawler) — see `../config.example.toml`.

## Run

```bash
# backfill: chunk + embed pages that don't have chunks yet (one-shot)
uv run python -m crowlr_py.backfill.run

# web server: search UI + streamed LLM summary
uv run uvicorn crowlr_py.web.app:app --host 0.0.0.0 --port 8080
```

Or via the root `Makefile`: `make py-install`, `make py-backfill`, `make py-web`.

## Tests

```bash
uv run pytest
```
