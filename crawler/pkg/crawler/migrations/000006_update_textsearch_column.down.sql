ALTER TABLE pages DROP COLUMN IF EXISTS textsearch;
DROP INDEX IF EXISTS textsearch_idx;

ALTER TABLE PAGES ADD COLUMN textsearch tsvector GENERATED ALWAYS AS (setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(url, '')), 'C') ||
        setweight(to_tsvector('english', coalesce(text, '')), 'B')) STORED;

CREATE INDEX textsearch_idx ON pages USING GIN (textsearch);
