# crowlr

web crawler with full-text search, built in golang.

## Overview

A web crawler is a tool designed to explore the internet automatically. Beginning with a set of starting web addresses, it visits each site, collects links found on those pages, and adds them to a queue for future visits. This cycle continues, allowing the crawler to find and catalog new websites over time.

crowlr works in a similar way: it visits multiple pages at once using a pool of workers, pays attention to robots.txt rules and waits between requests to avoid overloading sites, saves page data in PostgreSQL, and lets users search everything through an integrated full-text search interface.

## Screenshots

### Search Interface

![CROWLR Search Interface](assets/search-interface.png)

### Crawler Output

![CROWLR Crawler Output](assets/crawler-output.png)

## Architecture

```mermaid
flowchart TB
    Binary([crowlr]):::binary

    Binary -->|crawl| Seeds
    Binary -->|web| Search

    Seeds([seed URLs]) -->|push| Frontier

    Frontier[Frontier<br/><br/>- per-host BFS queues<br/>- seen-URL dedup<br/>- polite pop with per-host delay]

    Frontier -->|next eligible URL| Workers

    Workers[Worker Pool<br/><br/>- concurrent fetchers<br/>- robots.txt + sitemap support<br/>- configurable count and delay]

    Workers -->|HTML| Extract

    Extract[Extract<br/><br/>- title from title tag<br/>- outlinks from anchor hrefs<br/>- resolve and normalize URLs]

    Extract -->|new URLs| Frontier
    Extract -->|page| DB

    DB[(PostgreSQL<br/><br/>- url, title, html, outlinks<br/>- tsvector full-text index)]

    DB -->|full-text search| Search

    Search[Search UI<br/><br/>- HTMX<br/>- ts_rank_cd ranking<br/>- highlighted snippets]

    classDef binary stroke:#666,stroke-width:2px
```

## Features

- Concurrent crawling with configurable worker pool
- Respects robots.txt
- Per-host politeness delays
- URL normalization (scheme, host casing, default ports, fragments, dot segments)
- PostgreSQL storage with full-text search (weighted tsvector: title > url > content)
- Minimal search UI with HTMX

## Requirements

- Go 1.21+
- PostgreSQL 14+ (or Docker)

## Setup

```bash
# Clone
git clone https://github.com/devraulu/crowlr.git
cd crowlr

# Start PostgreSQL
docker compose up -d

# Configure
cp config.example.toml config.toml
cp seeds.example.txt seeds.txt
# Add seed URLs to seeds.txt

# Run crawler
make dev

# Run search UI (separate terminal)
make web
# Open http://localhost:8080

# Or use the binary directly
./tmp/crawler crawl
./tmp/crawler web --port 9000
```

## Configuration

See `config.example.toml` for all options.

| Option | Description | Default |
|--------|-------------|---------|
| `dsn` | PostgreSQL connection string | - |
| `crawler.workers` | Number of concurrent workers | `8` |
| `crawler.crawl_limit` | Max pages to crawl | `1000` |
| `crawler.user_agent` | User-Agent header | - |
| `politeness.delay` | Min delay between requests to same host | `1s` |
| `politeness.fetch_timeout` | Max duration for an individual fetch | `10s` |
| `logging.level` | Log level (debug, info, warn, error) | `info` |
| `logging.format` | Log format (text, json) | `json` |

## Project Structure

```
cmd/
  crawler/    # single binary — `crawl` and `web` subcommands
pkg/
  crawler/    # frontier, workers, postgres store, full-text search
  config/     # TOML configuration
  logger/     # structured logging (bunyan-compatible)
```

## License

MIT
