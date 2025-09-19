package seo

import (
	"net/url"
	"strings"
)

func CanonicalURL(base string, u *url.URL) string {
	if base == "" {
		return ""
	}
	v := *u
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

func XMLEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
