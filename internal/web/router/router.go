package router

import (
	"bytes"
	"database/sql"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"

	"liveRecord/internal/config"
	"liveRecord/internal/repo"
	"liveRecord/internal/security"
	"liveRecord/internal/seo"
	"liveRecord/internal/slug"
)

type Pager struct {
	Page       int
	TotalPages int
	PrevURL    string
	NextURL    string
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

func buildPager(c *gin.Context, page, totalPages int) Pager {
	path := c.Request.URL.Path
	q := c.Request.URL.Query()
	makeURL := func(n int) string { q.Set("page", strconv.Itoa(n)); return path + "?" + q.Encode() }
	p := Pager{Page: page, TotalPages: totalPages}
	if page > 1 {
		p.PrevURL = makeURL(page - 1)
	}
	if page < totalPages {
		p.NextURL = makeURL(page + 1)
	}
	return p
}

func New(db *sql.DB, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Static("/static", "internal/web/static")
	r.LoadHTMLGlob("internal/web/templates/**/*.tmpl")

	rp := repo.New(db)

	r.GET("/healthz", func(c *gin.Context) {
		if err := db.PingContext(c); err != nil {
			c.String(http.StatusServiceUnavailable, "db: %v", err)
			return
		}
		c.String(http.StatusOK, "ok")
	})

	r.GET("/", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		total, err := rp.CountPublished(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := rp.LatestPublishedPage(c, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/index.tmpl", gin.H{
			"Title":       cfg.Site.Title,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
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
		p, err := rp.FindBySlug(c, c.Param("slug"))
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
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Post":        p,
			"Content":     template.HTML(buf.String()),
		})
	})

	r.GET("/archive", func(c *gin.Context) {
		items, err := rp.AllForArchive(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		groups := make([]struct {
			Year  int
			Items []struct{ Title, Slug, Date string }
		}, 0)
		var cur int
		for i, it := range items {
			if i == 0 || it.Year != cur {
				groups = append(groups, struct {
					Year  int
					Items []struct{ Title, Slug, Date string }
				}{Year: it.Year})
				cur = it.Year
			}
			g := &groups[len(groups)-1]
			g.Items = append(g.Items, struct{ Title, Slug, Date string }{it.Title, it.Slug, it.Date.Format("2006-01-02")})
		}
		c.HTML(http.StatusOK, "pages/archive.tmpl", gin.H{
			"Title":       "归档",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Groups":      groups,
		})
	})

	r.GET("/categories", func(c *gin.Context) {
		cats, err := rp.CategoriesWithCount(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/categories.tmpl", gin.H{
			"Title":       "分类",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Categories":  cats,
		})
	})

	r.GET("/category/:slug", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		slugv := c.Param("slug")
		cat, total, err := rp.CategoryCount(c, slugv)
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
		posts, err := rp.CategoryPostsPage(c, slugv, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/category.tmpl", gin.H{
			"Title":       cat.Name,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Category":    cat, "Posts": posts, "Pager": buildPager(c, page, maxPage),
		})
	})

	r.GET("/tags", func(c *gin.Context) {
		tags, err := rp.TagsWithCount(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/tags.tmpl", gin.H{
			"Title":       "标签",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Tags":        tags,
		})
	})

	r.GET("/tag/:slug", func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		slugv := c.Param("slug")
		tag, total, err := rp.TagCount(c, slugv)
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
		posts, err := rp.TagPostsPage(c, slugv, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/tag.tmpl", gin.H{
			"Title":       tag.Name,
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"Tag":         tag, "Posts": posts, "Pager": buildPager(c, page, maxPage),
		})
	})

	// Admin auth helpers
	auth := func(c *gin.Context) {
		if _, _, ok := security.GetAuthCookie(c.Request, cfg.Security.SessionSecret); !ok {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		c.Next()
	}

	r.GET("/admin/login", func(c *gin.Context) {
		token := security.EnsureCSRFCookie(c.Writer, c.Request)
		c.HTML(http.StatusOK, "pages/admin_login.tmpl", gin.H{
			"Title":       "登录",
			"SiteTitle":   cfg.Site.Title,
			"Description": cfg.Site.Description,
			"Canonical":   seo.CanonicalURL(cfg.Site.BaseURL, c.Request.URL),
			"CSRFToken":   token,
		})
	})

	r.POST("/admin/login", func(c *gin.Context) {
		if !security.VerifyCSRF(c.Request) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		_ = c.Request.ParseForm()
		username := c.PostForm("username")
		password := c.PostForm("password")
		id, hash, err := rp.FindUserByUsername(c, username)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		if id == 0 {
			c.String(http.StatusUnauthorized, "invalid credentials")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			c.String(http.StatusUnauthorized, "invalid credentials")
			return
		}
		security.SetAuthCookie(c.Writer, cfg.Security.SessionSecret, id, username)
		c.Redirect(http.StatusFound, "/admin")
	})

	r.POST("/admin/logout", func(c *gin.Context) {
		security.ClearAuthCookie(c.Writer)
		c.Redirect(http.StatusFound, "/")
	})

	r.GET("/admin", auth, func(c *gin.Context) {
		c.HTML(http.StatusOK, "pages/admin_index.tmpl", gin.H{"Title": "管理后台", "SiteTitle": cfg.Site.Title})
	})

	r.GET("/admin/posts", auth, func(c *gin.Context) {
		page := parsePage(c)
		size := 10
		total, err := rp.AdminCountPosts(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		maxPage := maxInt(1, (total+size-1)/size)
		if page > maxPage {
			page = maxPage
		}
		posts, err := rp.AdminListPostsPage(c, size, (page-1)*size)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		c.HTML(http.StatusOK, "pages/admin_posts_list.tmpl", gin.H{
			"Title":     "文章管理",
			"SiteTitle": cfg.Site.Title,
			"Posts":     posts,
			"Pager":     buildPager(c, page, maxPage),
			"CSRFToken": security.EnsureCSRFCookie(c.Writer, c.Request),
		})
	})

	r.GET("/admin/posts/new", auth, func(c *gin.Context) {
		token := security.EnsureCSRFCookie(c.Writer, c.Request)
		c.HTML(http.StatusOK, "pages/admin_post_form.tmpl", gin.H{
			"Title":     "新建文章",
			"SiteTitle": cfg.Site.Title,
			"CSRFToken": token,
			"Action":    "/admin/posts",
			"Post":      repo.Post{},
		})
	})

	r.POST("/admin/posts", auth, func(c *gin.Context) {
		if !security.VerifyCSRF(c.Request) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		_ = c.Request.ParseForm()
		var p repo.Post
		p.Title = c.PostForm("title")
		p.Slug = c.PostForm("slug")
		p.Summary = c.PostForm("summary")
		p.ContentMD = c.PostForm("content_md")
		status := c.PostForm("status")
		if v := c.PostForm("published_at"); v != "" {
			if t, err := time.Parse("2006-01-02 15:04", v); err == nil {
				p.PublishedAt = t
			}
		}
		// slug normalize + unique
		base := slug.Normalize(p.Slug)
		if base == "" {
			base = slug.Normalize(p.Title)
		}
		u, err := rp.EnsureUniqueSlug(c, base)
		if err != nil {
			c.String(http.StatusBadRequest, "slug")
			return
		}
		p.Slug = u
		id, err := rp.AdminCreatePost(c, &p, status)
		if err != nil {
			c.String(http.StatusBadRequest, "save failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts/"+strconv.FormatInt(id, 10)+"/edit")
	})

	r.GET("/admin/posts/:id/edit", auth, func(c *gin.Context) {
		token := security.EnsureCSRFCookie(c.Writer, c.Request)
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		p, err := rp.AdminGetPost(c, id)
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
		if !security.VerifyCSRF(c.Request) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		_ = c.Request.ParseForm()
		var p repo.Post
		p.ID = id
		p.Title = c.PostForm("title")
		p.Slug = slug.Normalize(c.PostForm("slug"))
		if p.Slug == "" {
			p.Slug = slug.Normalize(p.Title)
		}
		p.Summary = c.PostForm("summary")
		p.ContentMD = c.PostForm("content_md")
		status := c.PostForm("status")
		p.PublishedAt = time.Time{}
		if v := c.PostForm("published_at"); v != "" {
			if t, err := time.Parse("2006-01-02 15:04", v); err == nil {
				p.PublishedAt = t
			}
		}
		if err := rp.AdminUpdatePost(c, &p, status); err != nil {
			c.String(http.StatusBadRequest, "update failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts")
	})

	r.POST("/admin/posts/:id/delete", auth, func(c *gin.Context) {
		if !security.VerifyCSRF(c.Request) {
			c.String(http.StatusBadRequest, "CSRF")
			return
		}
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if err := rp.AdminDeletePost(c, id); err != nil {
			c.String(http.StatusBadRequest, "delete failed")
			return
		}
		c.Redirect(http.StatusFound, "/admin/posts")
	})

	// feed + sitemap
	r.GET("/feed.xml", func(c *gin.Context) {
		items, err := rp.PublishedForFeed(c, 20)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		base := cfg.Site.BaseURL
		var sb bytes.Buffer
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel>")
		sb.WriteString("<title>")
		sb.WriteString(seo.XMLEscape(cfg.Site.Title))
		sb.WriteString("</title>")
		sb.WriteString("<link>")
		sb.WriteString(seo.XMLEscape(base))
		sb.WriteString("</link>")
		sb.WriteString("<description>")
		sb.WriteString(seo.XMLEscape(cfg.Site.Description))
		sb.WriteString("</description>")
		sb.WriteString("<lastBuildDate>")
		sb.WriteString(time.Now().Format(time.RFC1123Z))
		sb.WriteString("</lastBuildDate>")
		for _, it := range items {
			sb.WriteString("<item>")
			sb.WriteString("<title>")
			sb.WriteString(seo.XMLEscape(it.Title))
			sb.WriteString("</title>")
			link := strings.TrimRight(base, "/") + "/post/" + it.Slug
			sb.WriteString("<link>")
			sb.WriteString(seo.XMLEscape(link))
			sb.WriteString("</link>")
			sb.WriteString("<guid>")
			sb.WriteString(seo.XMLEscape(link))
			sb.WriteString("</guid>")
			if it.Summary != "" {
				sb.WriteString("<description>")
				sb.WriteString(seo.XMLEscape(it.Summary))
				sb.WriteString("</description>")
			}
			sb.WriteString("<pubDate>")
			sb.WriteString(it.PublishedAt.Format(time.RFC1123Z))
			sb.WriteString("</pubDate>")
			sb.WriteString("</item>")
		}
		sb.WriteString("</channel></rss>")
		c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", sb.Bytes())
	})

	r.GET("/sitemap.xml", func(c *gin.Context) {
		posts, err := rp.AllPublishedSlugs(c)
		if err != nil {
			c.String(http.StatusInternalServerError, "query failed")
			return
		}
		base := strings.TrimRight(cfg.Site.BaseURL, "/")
		var sb bytes.Buffer
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
		sb.WriteString("<url><loc>")
		sb.WriteString(seo.XMLEscape(base + "/"))
		sb.WriteString("</loc><changefreq>daily</changefreq><priority>0.8</priority></url>")
		for _, p := range posts {
			sb.WriteString("<url><loc>")
			sb.WriteString(seo.XMLEscape(base + "/post/" + p.Slug))
			sb.WriteString("</loc>")
			if !p.PublishedAt.IsZero() {
				sb.WriteString("<lastmod>")
				sb.WriteString(p.PublishedAt.Format("2006-01-02"))
				sb.WriteString("</lastmod>")
			}
			sb.WriteString("<changefreq>weekly</changefreq><priority>0.6</priority></url>")
		}
		sb.WriteString("</urlset>")
		c.Data(http.StatusOK, "application/xml; charset=utf-8", sb.Bytes())
	})

	return r
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
