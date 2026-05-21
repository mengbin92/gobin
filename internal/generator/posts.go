package generator

import (
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/mengbin92/gobin/internal/textutil"
)

func preparePosts(posts []*parser.Post, cfg *config.Config, buildDrafts bool) []*parser.Post {
	visiblePosts := make([]*parser.Post, 0, len(posts))

	for _, post := range posts {
		post.URL = buildPostURL(post, cfg)
		normalizeRenderedContent(cfg, &post.Content, &post.ContentHTML, &post.Summary, &post.SummaryHTML)
		if !isVisiblePost(post, buildDrafts) {
			continue
		}
		visiblePosts = append(visiblePosts, post)
	}

	return visiblePosts
}

func normalizeRenderedContent(cfg *config.Config, values ...*string) {
	baseURL := ""
	if cfg != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}

	replacer := strings.NewReplacer(
		"{{site.url}}", baseURL,
		"{{ site.url }}", baseURL,
		"{{site.baseurl}}", "",
		"{{ site.baseurl }}", "",
		"./{{site.url}}/", "/",
		"./{{ site.url }}/", "/",
	)

	for _, value := range values {
		if value == nil || *value == "" {
			continue
		}
		*value = replacer.Replace(*value)
	}
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
	return textutil.Slug(s)
}
