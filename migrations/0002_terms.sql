-- categories, tags, and post_tags
CREATE TABLE IF NOT EXISTS categories (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT
);

ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS category_id BIGINT NULL REFERENCES categories(id);

CREATE INDEX IF NOT EXISTS idx_posts_category_id ON posts(category_id);

CREATE TABLE IF NOT EXISTS tags (
    id   BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id  BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE(post_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_post_tags_tag_id ON post_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_post_tags_post_id ON post_tags(post_id);

-- seed a default category and assign existing posts
INSERT INTO categories (name, slug, description)
VALUES ('未分类', 'uncategorized', '默认分类')
ON CONFLICT (slug) DO NOTHING;

UPDATE posts SET category_id = (SELECT id FROM categories WHERE slug='uncategorized')
WHERE category_id IS NULL;

-- optional minimal tags seed
INSERT INTO tags (name, slug) VALUES
('随笔','essay'), ('技术','tech'), ('生活','life')
ON CONFLICT (slug) DO NOTHING;

-- link a few posts to tags if not already linked
INSERT INTO post_tags (post_id, tag_id)
SELECT p.id, t.id FROM posts p
JOIN tags t ON t.slug = CASE WHEN p.id % 3 = 0 THEN 'tech' WHEN p.id % 3 = 1 THEN 'life' ELSE 'essay' END
ON CONFLICT DO NOTHING;
