package repo

import (
	"context"
)

func (r *Repo) CategoriesWithCount(ctx context.Context) ([]Category, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT c.id, c.name, c.slug, COUNT(p.id) AS cnt
        FROM categories c
        LEFT JOIN posts p ON p.category_id = c.id AND p.status='published' AND p.published_at <= now()
        GROUP BY c.id, c.name, c.slug
        ORDER BY c.name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) CategoryCount(ctx context.Context, slug string) (*Category, int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id, name, slug FROM categories WHERE slug=$1`, slug)
	var c Category
	if err := row.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	row2 := r.DB.QueryRowContext(ctx, `
        SELECT COUNT(1) FROM posts WHERE category_id=$1 AND status='published' AND published_at <= now()`, c.ID)
	var n int
	if err := row2.Scan(&n); err != nil {
		return &c, 0, err
	}
	return &c, n, nil
}

func (r *Repo) CategoryPostsPage(ctx context.Context, slug string, limit, offset int) ([]Post, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id FROM categories WHERE slug=$1`, slug)
	var catID int64
	if err := row.Scan(&catID); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), published_at
        FROM posts
        WHERE category_id=$1 AND status='published' AND published_at <= now()
        ORDER BY published_at DESC
        LIMIT $2 OFFSET $3
    `, catID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repo) TagsWithCount(ctx context.Context) ([]Tag, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT t.id, t.name, t.slug, COUNT(p.id) AS cnt
        FROM tags t
        LEFT JOIN post_tags pt ON pt.tag_id = t.id
        LEFT JOIN posts p ON p.id = pt.post_id AND p.status='published' AND p.published_at <= now()
        GROUP BY t.id, t.name, t.slug
        ORDER BY t.name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) TagCount(ctx context.Context, slug string) (*Tag, int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id, name, slug FROM tags WHERE slug=$1`, slug)
	var t Tag
	if err := row.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	row2 := r.DB.QueryRowContext(ctx, `
        SELECT COUNT(1)
        FROM posts p JOIN post_tags pt ON pt.post_id=p.id
        WHERE pt.tag_id=$1 AND p.status='published' AND p.published_at <= now()`, t.ID)
	var n int
	if err := row2.Scan(&n); err != nil {
		return &t, 0, err
	}
	return &t, n, nil
}

func (r *Repo) TagPostsPage(ctx context.Context, slug string, limit, offset int) ([]Post, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id FROM tags WHERE slug=$1`, slug)
	var tagID int64
	if err := row.Scan(&tagID); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, `
        SELECT p.id, p.title, p.slug, COALESCE(p.summary,''), p.published_at
        FROM posts p
        JOIN post_tags pt ON pt.post_id = p.id
        WHERE pt.tag_id=$1 AND p.status='published' AND p.published_at <= now()
        ORDER BY p.published_at DESC
        LIMIT $2 OFFSET $3
    `, tagID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
