-- schema: minimal MVP for posts
CREATE TABLE IF NOT EXISTS posts (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT        NOT NULL,
    slug         TEXT        NOT NULL UNIQUE,
    summary      TEXT,
    content_md   TEXT,
    status       TEXT        NOT NULL DEFAULT 'published' CHECK (status IN ('draft','published')),
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_status_published_at ON posts (status, published_at DESC);

-- seed: 12 posts for homepage testing
INSERT INTO posts (title, slug, summary, content_md, status, published_at)
SELECT 'Post ' || gs::text,
       'post-' || gs::text,
       'Summary for post ' || gs::text,
       '# Heading for post ' || gs::text,
       'published',
       now() - make_interval(days => gs)
FROM generate_series(1, 12) AS gs
ON CONFLICT DO NOTHING;
