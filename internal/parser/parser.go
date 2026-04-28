package parser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// Post represents a blog post
type Post struct {
	// Front matter fields
	Title       string    `yaml:"title"`
	Date        time.Time `yaml:"date"`
	LastMod     time.Time `yaml:"lastmod"`
	Draft       bool      `yaml:"draft"`
	Published   *bool     `yaml:"published"`
	Description string    `yaml:"description"`
	Tags        []string  `yaml:"tags"`
	Categories  []string  `yaml:"categories"`
	Keywords    []string  `yaml:"keywords"`
	Slug        string    `yaml:"slug"`
	Aliases     []string  `yaml:"aliases"`
	Weight      int       `yaml:"weight"`
	Layout      string    `yaml:"layout"`

	// Internal fields
	FilePath    string                 `yaml:"-"`
	Content     string                 `yaml:"-"`
	ContentHTML string                 `yaml:"-"`
	WordCount   int                    `yaml:"-"`
	ReadingTime int                    `yaml:"-"`
	Summary     string                 `yaml:"-"`
	SummaryHTML string                 `yaml:"-"`
	URL         string                 `yaml:"-"`
	Section     string                 `yaml:"-"`
	Params      map[string]interface{} `yaml:"-"`
}

// Page represents a standalone markdown page.
type Page struct {
	Title       string                 `yaml:"title"`
	Description string                 `yaml:"description"`
	Layout      string                 `yaml:"layout"`
	Slug        string                 `yaml:"slug"`
	Permalink   string                 `yaml:"permalink"`
	FilePath    string                 `yaml:"-"`
	Content     string                 `yaml:"-"`
	ContentHTML string                 `yaml:"-"`
	URL         string                 `yaml:"-"`
	Params      map[string]interface{} `yaml:"-"`
}

// RenderOptions controls Markdown rendering behavior.
type RenderOptions struct {
	AllowUnsafeHTML bool
}

// DefaultRenderOptions returns parser settings that preserve current behavior.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{AllowUnsafeHTML: true}
}

type postFrontMatter struct {
	Title       string                 `yaml:"title"`
	Date        time.Time              `yaml:"date"`
	LastMod     time.Time              `yaml:"lastmod"`
	Draft       bool                   `yaml:"draft"`
	Published   *bool                  `yaml:"published"`
	Description string                 `yaml:"description"`
	Tags        yaml.Node              `yaml:"tags"`
	Categories  yaml.Node              `yaml:"categories"`
	Keywords    yaml.Node              `yaml:"keywords"`
	Slug        string                 `yaml:"slug"`
	Aliases     yaml.Node              `yaml:"aliases"`
	Weight      int                    `yaml:"weight"`
	Layout      string                 `yaml:"layout"`
	Params      map[string]interface{} `yaml:",inline"`
}

type pageFrontMatter struct {
	Title       string                 `yaml:"title"`
	Description string                 `yaml:"description"`
	Layout      string                 `yaml:"layout"`
	Slug        string                 `yaml:"slug"`
	Permalink   string                 `yaml:"permalink"`
	Params      map[string]interface{} `yaml:",inline"`
}

func normalizePostFrontMatter(raw postFrontMatter, path string, markdownContent string, renderedHTML string) (*Post, error) {
	tags, err := decodeStringListNode(raw.Tags)
	if err != nil {
		return nil, fmt.Errorf("invalid tags: %w", err)
	}
	categories, err := decodeStringListNode(raw.Categories)
	if err != nil {
		return nil, fmt.Errorf("invalid categories: %w", err)
	}
	keywords, err := decodeStringListNode(raw.Keywords)
	if err != nil {
		return nil, fmt.Errorf("invalid keywords: %w", err)
	}
	aliases, err := decodeStringListNode(raw.Aliases)
	if err != nil {
		return nil, fmt.Errorf("invalid aliases: %w", err)
	}

	post := &Post{
		Title:       raw.Title,
		Date:        raw.Date,
		LastMod:     raw.LastMod,
		Draft:       raw.Draft,
		Published:   raw.Published,
		Description: raw.Description,
		Tags:        tags,
		Categories:  categories,
		Keywords:    keywords,
		Slug:        raw.Slug,
		Aliases:     aliases,
		Weight:      raw.Weight,
		Layout:      raw.Layout,
		FilePath:    path,
		Content:     strings.TrimSpace(markdownContent),
		ContentHTML: renderedHTML,
		Params:      raw.Params,
	}

	if post.Date.IsZero() {
		post.Date = dateFromFilename(filepath.Base(path))
	}
	if post.Slug == "" {
		post.Slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if len(post.Slug) > 11 && post.Slug[10] == '-' {
			post.Slug = post.Slug[11:]
		}
	}
	post.URL = "/" + post.Slug + "/"
	post.WordCount = wordCount(markdownContent)
	post.ReadingTime = (post.WordCount + 200) / 250
	post.Summary = generateSummary(markdownContent)
	post.SummaryHTML = generateSummaryHTML(post.ContentHTML)
	if post.Layout == "" {
		post.Layout = "post"
	}

	return post, nil
}

func normalizePageFrontMatter(raw pageFrontMatter, path string, baseDir string, markdownContent string, renderedHTML string) (*Page, error) {
	page := &Page{
		Title:       raw.Title,
		Description: raw.Description,
		Layout:      raw.Layout,
		Slug:        raw.Slug,
		Permalink:   raw.Permalink,
		FilePath:    path,
		Content:     strings.TrimSpace(markdownContent),
		ContentHTML: renderedHTML,
		Params:      raw.Params,
	}

	if page.Slug == "" {
		page.Slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if page.Title == "" {
		page.Title = page.Slug
	}
	page.URL = pageURLForPath(path, baseDir, page)
	if page.Layout == "" {
		page.Layout = "page"
	}

	return page, nil
}

func decodeStringListNode(node yaml.Node) ([]string, error) {
	if node.Kind == 0 {
		return nil, nil
	}

	switch node.Kind {
	case yaml.SequenceNode:
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("expected scalar sequence item")
			}
			value := strings.TrimSpace(item.Value)
			if value != "" {
				values = append(values, value)
			}
		}
		return values, nil
	case yaml.ScalarNode:
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return nil, nil
		}

		if strings.Contains(value, ",") {
			parts := strings.Split(value, ",")
			values := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					values = append(values, part)
				}
			}
			return values, nil
		}

		return []string{value}, nil
	default:
		return nil, fmt.Errorf("expected scalar or sequence, got kind %d", node.Kind)
	}
}

// ParsePosts parses all markdown posts from a directory
func ParsePosts(dir string) ([]*Post, error) {
	return ParsePostsWithOptions(dir, DefaultRenderOptions())
}

// ParsePostsWithOptions parses all markdown posts from a directory using render options.
func ParsePostsWithOptions(dir string, opts RenderOptions) ([]*Post, error) {
	var posts []*Post

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		post, err := ParsePostWithOptions(path, opts)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if post != nil {
			posts = append(posts, post)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	return posts, nil
}

// ParsePages parses standalone markdown pages recursively from a directory.
func ParsePages(dir string) ([]*Page, error) {
	return ParsePagesWithOptions(dir, DefaultRenderOptions())
}

// ParsePagesWithOptions parses standalone markdown pages recursively using render options.
func ParsePagesWithOptions(dir string, opts RenderOptions) ([]*Page, error) {
	if dir == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var pages []*Page
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		page, err := ParsePageWithOptions(path, dir, opts)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		pages = append(pages, page)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return pages, nil
}

// ParsePost parses a single markdown post file
func ParsePost(path string) (*Post, error) {
	return ParsePostWithOptions(path, DefaultRenderOptions())
}

// ParsePostWithOptions parses a single markdown post file using render options.
func ParsePostWithOptions(path string, opts RenderOptions) (*Post, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Split front matter and content
	frontMatter, markdownContent, err := splitFrontMatter(string(content))
	if err != nil {
		return nil, err
	}

	var raw postFrontMatter
	if err := yaml.Unmarshal([]byte(frontMatter), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	// Render markdown to HTML
	renderedHTML, err := renderMarkdownWithOptions(markdownContent, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to render markdown: %w", err)
	}

	post, err := normalizePostFrontMatter(raw, path, markdownContent, renderedHTML)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// ParsePage parses a standalone markdown page.
func ParsePage(path string, baseDir string) (*Page, error) {
	return ParsePageWithOptions(path, baseDir, DefaultRenderOptions())
}

// ParsePageWithOptions parses a standalone markdown page using render options.
func ParsePageWithOptions(path string, baseDir string, opts RenderOptions) (*Page, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	frontMatter, markdownContent, err := splitFrontMatter(string(content))
	if err != nil {
		return nil, err
	}

	var raw pageFrontMatter
	if err := yaml.Unmarshal([]byte(frontMatter), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	renderedHTML, err := renderMarkdownWithOptions(markdownContent, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to render markdown: %w", err)
	}

	return normalizePageFrontMatter(raw, path, baseDir, markdownContent, renderedHTML)
}

var filenameDatePattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})-`)

func dateFromFilename(name string) time.Time {
	match := filenameDatePattern.FindStringSubmatch(name)
	if len(match) != 4 {
		return time.Time{}
	}

	dateValue, err := time.Parse("2006-01-02", strings.Join(match[1:], "-"))
	if err != nil {
		return time.Time{}
	}

	return dateValue
}

func renderMarkdown(markdownContent string) (string, error) {
	return renderMarkdownWithOptions(markdownContent, DefaultRenderOptions())
}

func renderMarkdownWithOptions(markdownContent string, opts RenderOptions) (string, error) {
	options := []goldmark.Option{}
	if opts.AllowUnsafeHTML {
		options = append(options, goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()))
	}

	md := goldmark.New(options...)

	var buf strings.Builder
	if err := md.Convert([]byte(markdownContent), &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func pageURLForPath(path, baseDir string, page *Page) string {
	if page != nil && strings.TrimSpace(page.Permalink) != "" {
		url := strings.TrimSpace(page.Permalink)
		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}
		if strings.HasSuffix(url, ".html") {
			return url
		}
		return strings.TrimRight(url, "/") + "/"
	}

	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		relPath = filepath.Base(path)
	}

	relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath))
	relPath = filepath.ToSlash(relPath)
	relPath = strings.Trim(relPath, "/")
	if relPath == "" {
		relPath = page.Slug
	}

	return "/" + relPath + "/"
}

// splitFrontMatter separates YAML front matter from markdown content
func splitFrontMatter(content string) (string, string, error) {
	lines := strings.Split(content, "\n")

	// Check for opening front matter delimiter
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, fmt.Errorf("missing front matter delimiter")
	}

	var frontMatterLines []string
	var markdownStart int

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			markdownStart = i + 1
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i])
	}

	if markdownStart == 0 {
		return "", content, fmt.Errorf("missing closing front matter delimiter")
	}

	return strings.Join(frontMatterLines, "\n"), strings.Join(lines[markdownStart:], "\n"), nil
}

// wordCount counts words in a string
func wordCount(s string) int {
	words := strings.Fields(s)
	return len(words)
}

// generateSummary generates a summary from the first paragraph
func generateSummary(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 200 {
				return line[:200] + "..."
			}
			return line
		}
	}
	return ""
}

// generateSummaryHTML generates HTML summary from first paragraph
func generateSummaryHTML(htmlContent string) string {
	// Simple HTML tag stripping for summary
	// In a real implementation, use proper HTML parsing
	text := strings.TrimSpace(htmlContent)
	if text == "" {
		return ""
	}

	// Get first paragraph
	idx := strings.Index(text, "</p>")
	if idx > 0 {
		return text[:idx+4]
	}

	// If no paragraph tag, truncate at reasonable length
	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}
