package repo

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

type Post struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	ContentMD   string
	PublishedAt time.Time
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
        ORDER BY created_at DESC
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
        INSERT INTO posts (title, slug, summary, content_md, status, published_at)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING id
    `, p.Title, p.Slug, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repo) AdminUpdatePost(ctx context.Context, p *Post, status string) error {
	_, err := r.DB.ExecContext(ctx, `
        UPDATE posts SET title=$1, slug=$2, summary=$3, content_md=$4, status=$5, published_at=$6, updated_at=now()
        WHERE id=$7
    `, p.Title, p.Slug, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt), p.ID)
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
