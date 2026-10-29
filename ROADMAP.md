# Roadmap

## 1. Distributed crawling

Move the crawler from one process to several workers sharing state.

- [ ] Move the frontier out of process memory (Postgres queue with `SELECT ... FOR UPDATE SKIP LOCKED`, or Redis)
- [ ] Shared seen-URL deduplication (DB unique index or Redis/Bloom filter)
- [ ] Assign hosts to workers (consistent hashing) so per-host politeness holds across nodes
- [ ] Distributed per-host rate limiting and shared robots.txt cache
- [ ] Worker registration, heartbeats and leases so URLs from dead workers get re-queued
- [ ] Separate `coordinator` and `worker` subcommands in the CLI
- [ ] Graceful shutdown and resume of an interrupted crawl
- [ ] Aggregate stats across workers
- [ ] docker-compose setup with multiple workers for local testing
- [ ] Acceptance tests with more than one worker

## 2. Support for the major LLM providers

Use LLMs on crawled pages (summaries, extraction, classification, embeddings for semantic search).

- [ ] Provider-agnostic `LLM` interface (chat/completion, embeddings, streaming)
- [ ] Anthropic (Claude) provider
- [ ] OpenAI provider
- [ ] Google Gemini provider
- [ ] Mistral provider
- [ ] OpenAI-compatible endpoints (Ollama, vLLM, OpenRouter, Groq, etc.)
- [ ] Optional AWS Bedrock / Azure OpenAI / Vertex AI support
- [ ] `[llm]` config section: provider, model, API key (from env), limits
- [ ] Retries, backoff and rate-limit handling per provider
- [ ] Token and cost tracking per crawl
- [ ] Page processing pipeline step (summary, structured extraction) stored in Postgres
- [ ] Embeddings stored with pgvector, plus hybrid search (full-text + semantic)
- [ ] Provider tests with recorded/mocked responses

## 3. Crawling UI

Start and manage crawls from the web UI instead of only from the CLI.

- [ ] API for crawl jobs (create, list, get, pause, resume, cancel)
- [ ] Crawl jobs table and per-job config (seeds, limits, workers, delay, allowed domains)
- [ ] New crawl form (seeds, limits, politeness, LLM options)
- [ ] Crawl list and detail pages
- [ ] Browse crawled pages per crawl (URL, title, status, outlinks, LLM output)
- [ ] Worker overview (active workers, status, current host)
- [ ] Authentication for the admin pages

## 4. Crawl progress

Show live progress for running crawls.

- [ ] Record per-crawl metrics (pages fetched, queued, failed, skipped by robots, bytes, pages/sec)
- [ ] Stream progress to the UI (SSE or WebSockets)
- [ ] Progress bar against `crawl_limit` and ETA
- [ ] Live log/event feed (recent fetches, errors)
- [ ] Per-host breakdown (queue depth, last fetch, errors)
- [ ] Error reporting by type (timeouts, 4xx/5xx, robots denied)
- [ ] Prometheus `/metrics` endpoint
