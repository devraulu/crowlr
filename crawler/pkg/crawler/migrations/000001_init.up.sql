CREATE TABLE IF NOT EXISTS pages (
    id         SERIAL PRIMARY KEY,
    url        TEXT UNIQUE NOT NULL,
    raw_url    TEXT NOT NULL,
    title TEXT NOT NULL,
    referrer   TEXT,
    status_code INTEGER,
    content TEXT NOT NULL,
    outlinks   JSONB,
    fetched_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
