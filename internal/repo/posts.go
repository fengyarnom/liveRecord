package repo

import (
	"context"
	"database/sql"
	"liveRecord/internal/slug"
	"strconv"
	"strings"
	"time"
)

type Post struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	ContentMD   string
	PublishedAt time.Time
	CategoryID  int64
}

type Category struct {
	ID    int64
	Name  string
	Slug  string
	Count int
}

type Tag struct {
	ID    int64
	Name  string
	Slug  string
	Count int
}

type ArchiveItem struct {
	Year  int
	ID    int64
	Title string
	Slug  string
	Date  time.Time
}

type Repo struct{ DB *sql.DB }

func New(db *sql.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) LatestPublished(ctx context.Context, limit int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary, ''), published_at
        FROM posts
        WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
        LIMIT $1
    `, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) CountPublished(ctx context.Context) (int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts WHERE status='published' AND published_at <= now()`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repo) LatestPublishedPage(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary, ''), published_at
        FROM posts
        WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) LatestPublishedWithContentPage(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary, ''), COALESCE(content_md,''), published_at
        FROM posts
        WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.ContentMD, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) FindBySlug(ctx context.Context, slug string) (*Post, error) {
	row := r.DB.QueryRowContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), COALESCE(content_md,''), published_at
        FROM posts
        WHERE status='published' AND published_at <= now() AND slug=$1
        LIMIT 1
    `, slug)
	var p Post
	if err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.ContentMD, &p.PublishedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repo) AllForArchive(ctx context.Context) ([]ArchiveItem, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT EXTRACT(YEAR FROM published_at)::int as y, id, title, slug, published_at
        FROM posts
        WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchiveItem
	for rows.Next() {
		var it ArchiveItem
		if err := rows.Scan(&it.Year, &it.ID, &it.Title, &it.Slug, &it.Date); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repo) PublishedForFeed(ctx context.Context, limit int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), published_at
        FROM posts
        WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
        LIMIT $1
    `, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) AllPublishedSlugs(ctx context.Context) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), COALESCE(published_at,to_timestamp(0))
        FROM posts WHERE status='published' AND published_at <= now()
        ORDER BY published_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Admin helpers

func (r *Repo) AdminCountPosts(ctx context.Context) (int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repo) AdminListPostsPage(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), COALESCE(content_md,''), COALESCE(published_at, to_timestamp(0))
        FROM posts
        ORDER BY COALESCE(published_at, to_timestamp(0)) DESC, id DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.ContentMD, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) AdminGetPost(ctx context.Context, id int64) (*Post, error) {
	row := r.DB.QueryRowContext(ctx, `
        SELECT id, title, slug, COALESCE(summary,''), COALESCE(content_md,''), COALESCE(published_at, to_timestamp(0))
        FROM posts WHERE id=$1`, id)
	var p Post
	if err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Summary, &p.ContentMD, &p.PublishedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repo) slugExists(ctx context.Context, slug string) (bool, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT 1 FROM posts WHERE slug=$1 LIMIT 1`, slug)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repo) EnsureUniqueSlug(ctx context.Context, base string) (string, error) {
	s := base
	i := 2
	for {
		ok, err := r.slugExists(ctx, s)
		if err != nil {
			return s, err
		}
		if !ok {
			return s, nil
		}
		s = base + "-" + strconv.Itoa(i)
		i++
		if i > 1000 {
			return s, nil
		}
	}
}

func (r *Repo) AdminCreatePost(ctx context.Context, p *Post, status string) (int64, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `
        INSERT INTO posts (title, slug, summary, content_md, status, published_at, category_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id
    `, p.Title, p.Slug, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt), nullableID(p.CategoryID)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repo) AdminUpdatePost(ctx context.Context, p *Post, status string) error {
	_, err := r.DB.ExecContext(ctx, `
        UPDATE posts SET title=$1, slug=$2, summary=$3, content_md=$4, status=$5, published_at=$6, category_id=$7
        WHERE id=$8
    `, p.Title, p.Slug, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt), nullableID(p.CategoryID), p.ID)
	return err
}

func (r *Repo) AdminDeletePost(ctx context.Context, id int64) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM posts WHERE id=$1`, id)
	return err
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullableID(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}

// CategoryIDBySlug returns category id by slug or 0 if not found.
func (r *Repo) CategoryIDBySlug(ctx context.Context, slug string) (int64, error) {
	if slug == "" {
		return 0, nil
	}
	row := r.DB.QueryRowContext(ctx, `SELECT id FROM categories WHERE slug=$1`, slug)
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}

// UpdatePostTags replaces post's tags with the given list of tag names.
// It ensures tags exist (by slug unique) and then rewrites post_tags.
func (r *Repo) UpdatePostTags(ctx context.Context, postID int64, tagNames []string) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id=$1`, postID); err != nil {
		return err
	}
	// Deduplicate input
	seen := map[string]struct{}{}
	for _, name := range tagNames {
		n := strings.TrimSpace(name)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		slugv := slug.Normalize(n)
		var tagID int64
		// Upsert tag and get id
		if err = tx.QueryRowContext(ctx, `
            INSERT INTO tags (name, slug)
            VALUES ($1,$2)
            ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name
            RETURNING id
        `, n, slugv).Scan(&tagID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO post_tags (post_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, postID, tagID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
