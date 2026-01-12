CREATE TABLE IF NOT EXISTS pages (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title TEXT,
    content TEXT,
    html TEXT,
    status_code INTEGER,
    outlinks JSONB
);

CREATE TABLE IF NOT EXISTS sitemaps (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    last_checked TIMESTAMPTZ,
    status_code INTEGER,
    content TEXT
);
