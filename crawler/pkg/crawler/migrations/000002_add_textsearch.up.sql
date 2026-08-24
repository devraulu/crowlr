ALTER TABLE pages ADD COLUMN textsearch tsvector GENERATED ALWAYS AS (setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
                          setweight(to_tsvector('english', coalesce(content, '')), 'B')) STORED;

CREATE INDEX textsearch_idx ON pages USING GIN (textsearch);
