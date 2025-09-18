package router

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// addLazyToImages parses an HTML fragment and ensures all <img> tags
// include lazy-loading friendly attributes. Returns the updated HTML.
func addLazyToImages(in string) string {
	if in == "" || !strings.Contains(in, "<img") {
		return in
	}
	// Parse as a full document to leverage the HTML parser, then
	// extract the body children when rendering back.
	doc, err := html.Parse(strings.NewReader(in))
	if err != nil {
		return in
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			ensureAttr(n, "loading", "lazy")
			ensureAttr(n, "decoding", "async")
			// Lower priority for non-critical images in lists/posts
			ensureAttr(n, "fetchpriority", "low")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return renderBodyInnerHTML(doc, in)
}

// stripImages removes all <img> tags from an HTML fragment.
func stripImages(in string) string {
	if in == "" || !strings.Contains(in, "<img") {
		return in
	}
	doc, err := html.Parse(strings.NewReader(in))
	if err != nil {
		return in
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "img" {
				// Remove c from the tree safely.
				removeNode(c)
				// continue without advancing c (it has been removed),
				// but NextSibling is already captured in for-loop progression.
				continue
			}
			walk(c)
		}
	}
	walk(doc)
	return renderBodyInnerHTML(doc, in)
}

func ensureAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if strings.EqualFold(n.Attr[i].Key, key) {
			// Already present; leave as-is
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func removeNode(n *html.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	p := n.Parent
	if p.FirstChild == n {
		p.FirstChild = n.NextSibling
	}
	if p.LastChild == n {
		p.LastChild = n.PrevSibling
	}
	if n.PrevSibling != nil {
		n.PrevSibling.NextSibling = n.NextSibling
	}
	if n.NextSibling != nil {
		n.NextSibling.PrevSibling = n.PrevSibling
	}
	n.Parent = nil
	n.PrevSibling = nil
	n.NextSibling = nil
}

// renderBodyInnerHTML renders the children of the <body> element if present,
// otherwise falls back to rendering the whole document. Returns original input
// on render error.
func renderBodyInnerHTML(doc *html.Node, fallback string) string {
	// Find <body>
	var body *html.Node
	var find func(n *html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil && body == nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	var buf bytes.Buffer
	if body != nil {
		for c := body.FirstChild; c != nil; c = c.NextSibling {
			if err := html.Render(&buf, c); err != nil {
				return fallback
			}
		}
		return buf.String()
	}
	if err := html.Render(&buf, doc); err != nil {
		return fallback
	}
	return buf.String()
}
