package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	pinyin "github.com/mozillazg/go-pinyin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"
	"strings"

	"liveRecord/internal/config"
)

type Post struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	ContentMD   string
	PublishedAt time.Time
}

type PostRepo struct{ DB *sql.DB }

func (r *PostRepo) LatestPublished(ctx context.Context, limit int) ([]Post, error) {
	rows, err := r.DB.QueryContext(ctx, `
        SELECT id, title, slug, COALESCE(summary, ''), published_at
        FROM posts
        WHERE status = 'published' AND published_at <= now()
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

func (r *PostRepo) PublishedForFeed(ctx context.Context, limit int) ([]Post, error) {
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

func (r *PostRepo) AllPublishedSlugs(ctx context.Context) ([]Post, error) {
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

func (r *PostRepo) CountPublished(ctx context.Context) (int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts WHERE status='published' AND published_at <= now()`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *PostRepo) LatestPublishedPage(ctx context.Context, limit, offset int) ([]Post, error) {
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

func (r *PostRepo) FindBySlug(ctx context.Context, slug string) (*Post, error) {
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

func (r *PostRepo) CategoriesWithCount(ctx context.Context) ([]Category, error) {
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

func (r *PostRepo) CategoryCount(ctx context.Context, slug string) (*Category, int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id, name, slug FROM categories WHERE slug=$1`, slug)
	var c Category
	if err := row.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
		if err == sql.ErrNoRows {
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

func (r *PostRepo) CategoryPostsPage(ctx context.Context, slug string, limit, offset int) ([]Post, error) {
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

func (r *PostRepo) TagsWithCount(ctx context.Context) ([]Tag, error) {
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

func (r *PostRepo) TagCount(ctx context.Context, slug string) (*Tag, int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id, name, slug FROM tags WHERE slug=$1`, slug)
	var t Tag
	if err := row.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
		if err == sql.ErrNoRows {
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

func (r *PostRepo) TagPostsPage(ctx context.Context, slug string, limit, offset int) ([]Post, error) {
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

type ArchiveItem struct {
	Year  int
	ID    int64
	Title string
	Slug  string
	Date  time.Time
}

func (r *PostRepo) AllForArchive(ctx context.Context) ([]ArchiveItem, error) {
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

func (r *PostRepo) AdminCountPosts(ctx context.Context) (int, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *PostRepo) AdminListPostsPage(ctx context.Context, limit, offset int) ([]Post, error) {
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

func (r *PostRepo) AdminGetPost(ctx context.Context, id int64) (*Post, error) {
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

func (r *PostRepo) slugExists(ctx context.Context, slug string) (bool, error) {
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

func (r *PostRepo) ensureUniqueSlug(ctx context.Context, base string) (string, error) {
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

func (r *PostRepo) AdminCreatePost(ctx context.Context, p *Post, status string) (int64, error) {
	slug := normalizeSlug(p.Slug)
	if slug == "" {
		slug = normalizeSlug(p.Title)
	}
	u, err := r.ensureUniqueSlug(ctx, slug)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.DB.QueryRowContext(ctx, `
        INSERT INTO posts (title, slug, summary, content_md, status, published_at)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING id
    `, p.Title, u, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *PostRepo) AdminUpdatePost(ctx context.Context, p *Post, status string) error {
	if p.Slug == "" {
		p.Slug = p.Title
	}
	p.Slug = normalizeSlug(p.Slug)
	_, err := r.DB.ExecContext(ctx, `
        UPDATE posts SET title=$1, slug=$2, summary=$3, content_md=$4, status=$5, published_at=$6, updated_at=now()
        WHERE id=$7
    `, p.Title, p.Slug, p.Summary, p.ContentMD, status, nullableTime(p.PublishedAt), p.ID)
	return err
}

func (r *PostRepo) AdminDeletePost(ctx context.Context, id int64) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM posts WHERE id=$1`, id)
	return err
}

func findUserByUsername(ctx context.Context, db *sql.DB, username string) (int64, string, error) {
	row := db.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username=$1 AND is_active=TRUE LIMIT 1`, username)
	var id int64
	var hash string
	switch err := row.Scan(&id, &hash); err {
	case nil:
		return id, hash, nil
	case sql.ErrNoRows:
		return 0, "", nil
	default:
		return 0, "", err
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	switch cfg.Server.GinMode {
	case gin.ReleaseMode:
		gin.SetMode(gin.ReleaseMode)
	case gin.DebugMode:
		gin.SetMode(gin.DebugMode)
	case gin.TestMode:
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}

	log.Printf("connecting to db: %s", cfg.Database.URL)
	log.Printf("site title: %s", cfg.Site.Title)
	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		log.Fatal(err)
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			log.Fatal("db ping failed: ", err)
		}
	}
	log.Printf("db connected")

	repo := &PostRepo{DB: db}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.Static("/static", "internal/web/static")
	r.LoadHTMLGlob("internal/web/templates/**/*.tmpl")

	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		if err := repo.DB.PingContext(ctx); err != nil {
			c.String(http.StatusServiceUnavailable, "db: %v", err)
			return
		}
		c.String(http.StatusOK, "ok")
	})

	r.GET("/", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		total, err := repo.CountPublished(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := repo.LatestPublishedPage(c, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/index.tmpl", gin.H{
			"Title":       cfg.Site.Title,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Posts":       posts,
			"Pager":       buildPager(c, page, maxPage),
		})
	})

	md := goldmark.New(
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithXHTML(), html.WithUnsafe()),
		goldmark.WithExtensions(extension.GFM, extension.Linkify),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	r.GET("/post/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		p, err := repo.FindBySlug(c, slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if p == nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		var buf bytes.Buffer
		if err := md.Convert([]byte(p.ContentMD), &buf); err != nil {
			c.String(http.StatusInternalServerError, "render failed")
			return
		}
		desc := p.Summary
		if desc == "" {
			desc = cfg.Site.Description
		}
		c.HTML(http.StatusOK, "pages/post.tmpl", gin.H{
			"Title":       p.Title,
			"SiteTitle":   cfg.Site.Title,
			"Description": desc,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Post":        p,
			"Content":     template.HTML(buf.String()),
		})
	})

	r.GET("/admin/login", func(c *gin.Context) {
		token := ensureCSRFCookie(c)
		c.HTML(http.StatusOK, "pages/admin_login.tmpl", gin.H{
			"Title":       "登录",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"CSRFToken":   token,
		})
	})

	r.POST("/admin/login", func(c *gin.Context) {
		if !verifyCSRF(c) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "bad form")
			return
		}
		username := strings.TrimSpace(c.PostForm("username"))
		password := c.PostForm("password")
		u, hash, err := findUserByUsername(c, repo.DB, username)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if u == 0 {
			c.String(http.StatusUnauthorized, "invalid credentials")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			c.String(http.StatusUnauthorized, "invalid credentials")
			return
		}
		setAuthCookie(c, cfg.Security.SessionSecret, u, username)
		c.Redirect(http.StatusFound, "/admin")
	})

	r.POST("/admin/logout", func(c *gin.Context) {
		clearAuthCookie(c)
		c.Redirect(http.StatusFound, "/")
	})

	auth := func(c *gin.Context) {
		if _, _, ok := getAuthCookie(c, cfg.Security.SessionSecret); !ok {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		c.Next()
	}

	r.GET("/admin", auth, func(c *gin.Context) {
		c.HTML(http.StatusOK, "pages/admin_index.tmpl", gin.H{"Title": "管理后台", "SiteTitle": cfg.Site.Title})
	})

	r.GET("/admin/posts", auth, func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		total, err := repo.AdminCountPosts(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := repo.AdminListPostsPage(c, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/admin_posts_list.tmpl", gin.H{
			"Title":     "文章管理",
			"SiteTitle": cfg.Site.Title,
			"Posts":     posts,
			"Pager":     buildPager(c, page, maxPage),
			"CSRFToken": ensureCSRFCookie(c),
		})
	})

	r.GET("/admin/posts/new", auth, func(c *gin.Context) {
		token := ensureCSRFCookie(c)
		c.HTML(http.StatusOK, "pages/admin_post_form.tmpl", gin.H{
			"Title":     "新建文章",
			"SiteTitle": cfg.Site.Title,
			"CSRFToken": token,
			"Action":    "/admin/posts",
			"Post":      Post{},
		})
	})

	r.POST("/admin/posts", auth, func(c *gin.Context) {
		if !verifyCSRF(c) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		_ = c.Request.ParseForm()
		var p Post
		p.Title = strings.TrimSpace(c.PostForm("title"))
		p.Slug = strings.TrimSpace(c.PostForm("slug"))
		p.Summary = strings.TrimSpace(c.PostForm("summary"))
		p.ContentMD = c.PostForm("content_md")
		status := c.PostForm("status")
		if v := strings.TrimSpace(c.PostForm("published_at")); v != "" {
			if t, err := time.Parse("2006-01-02 15:04", v); err == nil {
				p.PublishedAt = t
			}
		}
		id, err := repo.AdminCreatePost(c, &p, status)
		if err != nil {
			c.String(http.StatusBadRequest, "save failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts/"+strconv.FormatInt(id, 10)+"/edit")
	})

	r.GET("/admin/posts/:id/edit", auth, func(c *gin.Context) {
		token := ensureCSRFCookie(c)
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		p, err := repo.AdminGetPost(c, id)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if p == nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.HTML(http.StatusOK, "pages/admin_post_form.tmpl", gin.H{
			"Title":     "编辑文章",
			"SiteTitle": cfg.Site.Title,
			"CSRFToken": token,
			"Action":    "/admin/posts/" + c.Param("id"),
			"Post":      p,
		})
	})

	r.POST("/admin/posts/:id", auth, func(c *gin.Context) {
		if !verifyCSRF(c) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		_ = c.Request.ParseForm()
		var p Post
		p.ID = id
		p.Title = strings.TrimSpace(c.PostForm("title"))
		p.Slug = strings.TrimSpace(c.PostForm("slug"))
		p.Summary = strings.TrimSpace(c.PostForm("summary"))
		p.ContentMD = c.PostForm("content_md")
		status := c.PostForm("status")
		p.PublishedAt = time.Time{}
		if v := strings.TrimSpace(c.PostForm("published_at")); v != "" {
			if t, err := time.Parse("2006-01-02 15:04", v); err == nil {
				p.PublishedAt = t
			}
		}
		if err := repo.AdminUpdatePost(c, &p, status); err != nil {
			c.String(http.StatusBadRequest, "update failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts")
	})

	r.POST("/admin/posts/:id/delete", auth, func(c *gin.Context) {
		if !verifyCSRF(c) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if err := repo.AdminDeletePost(c, id); err != nil {
			c.String(http.StatusBadRequest, "delete failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts")
	})

	r.GET("/archive", func(c *gin.Context) {
		items, err := repo.AllForArchive(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		groups := make([]struct {
			Year  int
			Items []struct {
				Title string
				Slug  string
				Date  string
			}
		}, 0)
		var curYear int
		for i, it := range items {
			if i == 0 || it.Year != curYear {
				groups = append(groups, struct {
					Year  int
					Items []struct {
						Title string
						Slug  string
						Date  string
					}
				}{Year: it.Year})
				curYear = it.Year
			}
			g := &groups[len(groups)-1]
			g.Items = append(g.Items, struct{ Title, Slug, Date string }{Title: it.Title, Slug: it.Slug, Date: it.Date.Format("2006-01-02")})
		}
		c.HTML(http.StatusOK, "pages/archive.tmpl", gin.H{
			"Title":       "归档",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Groups":      groups,
		})
	})

	r.GET("/categories", func(c *gin.Context) {
		cats, err := repo.CategoriesWithCount(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/categories.tmpl", gin.H{
			"Title":       "分类",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Categories":  cats,
		})
	})

	r.GET("/category/:slug", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		slug := c.Param("slug")
		cat, total, err := repo.CategoryCount(c, slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if cat == nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := repo.CategoryPostsPage(c, slug, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/category.tmpl", gin.H{
			"Title":       cat.Name,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Category":    cat, "Posts": posts, "Pager": buildPager(c, page, maxPage),
		})
	})

	r.GET("/tags", func(c *gin.Context) {
		tags, err := repo.TagsWithCount(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/tags.tmpl", gin.H{
			"Title":       "标签",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Tags":        tags,
		})
	})

	r.GET("/tag/:slug", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		slug := c.Param("slug")
		tag, total, err := repo.TagCount(c, slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if tag == nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := repo.TagPostsPage(c, slug, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/tag.tmpl", gin.H{
			"Title":       tag.Name,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   canonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Tag":         tag, "Posts": posts, "Pager": buildPager(c, page, maxPage),
		})
	})

	r.GET("/feed.xml", func(c *gin.Context) {
		items, err := repo.PublishedForFeed(c, 20)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		base := strings.TrimRight(cfg.Site.BaseURL, "/")
		var sb strings.Builder
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		sb.WriteString("<rss version=\"2.0\"><channel>")
		sb.WriteString("<title>")
		sb.WriteString(xmlEscape(cfg.Site.Title))
		sb.WriteString("</title>")
		sb.WriteString("<link>")
		sb.WriteString(xmlEscape(base))
		sb.WriteString("</link>")
		sb.WriteString("<description>")
		sb.WriteString(xmlEscape(cfg.Site.Description))
		sb.WriteString("</description>")
		sb.WriteString("<lastBuildDate>")
		sb.WriteString(time.Now().Format(time.RFC1123Z))
		sb.WriteString("</lastBuildDate>")
		for _, it := range items {
			sb.WriteString("<item>")
			sb.WriteString("<title>")
			sb.WriteString(xmlEscape(it.Title))
			sb.WriteString("</title>")
			link := base + "/post/" + it.Slug
			sb.WriteString("<link>")
			sb.WriteString(xmlEscape(link))
			sb.WriteString("</link>")
			sb.WriteString("<guid>")
			sb.WriteString(xmlEscape(link))
			sb.WriteString("</guid>")
			if it.Summary != "" {
				sb.WriteString("<description>")
				sb.WriteString(xmlEscape(it.Summary))
				sb.WriteString("</description>")
			}
			sb.WriteString("<pubDate>")
			sb.WriteString(it.PublishedAt.Format(time.RFC1123Z))
			sb.WriteString("</pubDate>")
			sb.WriteString("</item>")
		}
		sb.WriteString("</channel></rss>")
		c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", []byte(sb.String()))
	})

	r.GET("/sitemap.xml", func(c *gin.Context) {
		posts, err := repo.AllPublishedSlugs(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		base := strings.TrimRight(cfg.Site.BaseURL, "/")
		var sb strings.Builder
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		sb.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
		// home
		sb.WriteString("<url><loc>")
		sb.WriteString(xmlEscape(base + "/"))
		sb.WriteString("</loc><changefreq>daily</changefreq><priority>0.8</priority></url>")
		for _, p := range posts {
			sb.WriteString("<url><loc>")
			sb.WriteString(xmlEscape(base + "/post/" + p.Slug))
			sb.WriteString("</loc>")
			if !p.PublishedAt.IsZero() {
				sb.WriteString("<lastmod>")
				sb.WriteString(p.PublishedAt.Format("2006-01-02"))
				sb.WriteString("</lastmod>")
			}
			sb.WriteString("<changefreq>weekly</changefreq><priority>0.6</priority></url>")
		}
		sb.WriteString("</urlset>")
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(sb.String()))
	})

	if _, err := strconv.Atoi(cfg.Server.Port); err != nil {
		log.Fatal("invalid PORT")
	}
	log.Printf("starting server on :%s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal(err)
	}
}

func parsePage(c *gin.Context) int {
	p := c.Query("page")
	if p == "" {
		return 1
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 {
		return 1
	}
	if n > 100000 {
		return 1
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Pager struct {
	Page       int
	TotalPages int
	PrevURL    string
	NextURL    string
}

func buildPager(c *gin.Context, page, totalPages int) Pager {
	path := c.Request.URL.Path
	q := c.Request.URL.Query()
	makeURL := func(n int) string {
		q.Set("page", strconv.Itoa(n))
		return path + "?" + q.Encode()
	}
	p := Pager{Page: page, TotalPages: totalPages}
	if page > 1 {
		p.PrevURL = makeURL(page - 1)
	}
	if page < totalPages {
		p.NextURL = makeURL(page + 1)
	}
	return p
}

func canonicalURL(base string, u *url.URL) string {
	if base == "" {
		return ""
	}
	v := *u
	// include page for >1
	if v.Query().Get("page") == "1" {
		q := v.Query()
		q.Del("page")
		v.RawQuery = q.Encode()
	}
	if v.RawQuery == "" {
		return strings.TrimRight(base, "/") + v.Path
	}
	return strings.TrimRight(base, "/") + v.Path + "?" + v.RawQuery
}

func ensureCSRFCookie(c *gin.Context) string {
	if v, err := c.Request.Cookie("csrf"); err == nil && v.Value != "" {
		return v.Value
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(c.Writer, &http.Cookie{Name: "csrf", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return tok
}

func verifyCSRF(c *gin.Context) bool {
	form := c.PostForm("_csrf")
	cv, err := c.Request.Cookie("csrf")
	if err != nil || form == "" {
		return false
	}
	if len(form) != len(cv.Value) {
		return false
	}
	// constant-time compare
	var same byte
	for i := 0; i < len(form); i++ {
		same |= form[i] ^ cv.Value[i]
	}
	return same == 0
}

func setAuthCookie(c *gin.Context, secret string, uid int64, username string) {
	// payload: uid|ts|username|nonce
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawURLEncoding.EncodeToString(b)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := []byte(strconv.FormatInt(uid, 10) + "|" + ts + "|" + username + "|" + nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := mac.Sum(nil)
	val := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
	http.SetCookie(c.Writer, &http.Cookie{Name: "auth", Value: val, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func getAuthCookie(c *gin.Context, secret string) (int64, string, bool) {
	ck, err := c.Request.Cookie("auth")
	if err != nil || ck.Value == "" {
		return 0, "", false
	}
	parts := strings.SplitN(ck.Value, ".", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	sig, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	if err1 != nil || err2 != nil {
		return 0, "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	exp := mac.Sum(nil)
	if !hmac.Equal(exp, sig) {
		return 0, "", false
	}
	// parse uid|ts|username|nonce
	fields := strings.Split(string(payload), "|")
	if len(fields) < 4 {
		return 0, "", false
	}
	uid, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	ts, _ := strconv.ParseInt(fields[1], 10, 64)
	if time.Since(time.Unix(ts, 0)) > 7*24*time.Hour {
		return 0, "", false
	}
	return uid, fields[2], true
}

func clearAuthCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "auth", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
}

func simpleSlugify(s string) string {
	return normalizeSlug(s)
}

func normalizeSlug(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// convert Han to pinyin (no tone)
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
	var b strings.Builder
	for _, r := range s {
		if r > 127 { // possibly CJK or other unicode
			pys := pinyin.SinglePinyin(r, a)
			if len(pys) > 0 {
				b.WriteString(pys[0])
				b.WriteByte(' ')
				continue
			}
			// non-mapped unicode -> space
			b.WriteByte(' ')
			continue
		}
		ch := byte(r)
		if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == ' ' || ch == '-' {
			b.WriteByte(ch)
		} else {
			b.WriteByte(' ')
		}
	}
	out := b.String()
	out = strings.TrimSpace(out)
	out = strings.ReplaceAll(out, " ", "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	return out
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
