CREATE TABLE crawl_jobs (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    seeds_count INTEGER NOT NULL,
    seeds JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error TEXT,
    stats JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX crawl_jobs_created_at_idx ON crawl_jobs (created_at DESC);
