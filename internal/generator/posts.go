package generator

import (
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func preparePosts(posts []*parser.Post, cfg *config.Config, buildDrafts bool) []*parser.Post {
	visiblePosts := make([]*parser.Post, 0, len(posts))

	for _, post := range posts {
		post.URL = buildPostURL(post, cfg)
		if !isVisiblePost(post, buildDrafts) {
			continue
		}
		visiblePosts = append(visiblePosts, post)
	}

	return visiblePosts
}

func isVisiblePost(post *parser.Post, buildDrafts bool) bool {
	if post == nil {
		return false
	}
	if post.Published != nil && !*post.Published {
		return false
	}
	if post.Draft && !buildDrafts {
		return false
	}
	return true
}

func buildPostURL(post *parser.Post, cfg *config.Config) string {
	pattern := "/:slug/"
	if cfg != nil && cfg.Permalinks != nil && cfg.Permalinks["posts"] != "" {
		pattern = cfg.Permalinks["posts"]
	}

	slug := urlize(post.Slug)
	if slug == "untitled" && post.Title != "" {
		slug = urlize(post.Title)
	}

	replacer := strings.NewReplacer(
		":year", post.Date.Format("2006"),
		":month", post.Date.Format("01"),
		":day", post.Date.Format("02"),
		":title", urlize(post.Title),
		":slug", slug,
	)

	url := replacer.Replace(pattern)
	url = strings.TrimSpace(url)
	if url == "" {
		url = "/" + slug + "/"
	}
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	url = strings.ReplaceAll(url, "//", "/")
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	return url
}

// urlize converts a string to URL-friendly format.
func urlize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			(r >= 0x4e00 && r <= 0x9fff) ||
			(r >= 0x3400 && r <= 0x4dbf) ||
			(r >= 0x20000 && r <= 0x2a6df) {
			result.WriteRune(r)
		}
	}

	url := result.String()
	for strings.Contains(url, "--") {
		url = strings.ReplaceAll(url, "--", "-")
	}

	url = strings.Trim(url, "-")
	if url == "" {
		return "untitled"
	}

	return url
}
