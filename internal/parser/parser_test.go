package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestParsePost tests parsing a single post with valid frontmatter
func TestParsePost(t *testing.T) {
	// Create a temporary markdown file
	tmpDir := t.TempDir()
	postContent := `---
title: "Test Post"
date: 2023-12-25T10:00:00+08:00
description: "Test description"
tags: ["test", "golang"]
categories: ["tech"]
draft: false
---

# Test Title

This is a test post content with more words to ensure proper reading time calculation. The content needs to be long enough to have a meaningful word count that results in a non-zero reading time. Adding more sentences here to increase the word count significantly for the test to pass properly. This should provide enough content for the reading time calculation to work correctly.

Here's a second paragraph with more content. This additional paragraph adds even more words to the post content to ensure that the word count is substantial enough for the reading time algorithm to calculate a non-zero value. The reading time calculation uses a formula based on word count, so we need sufficient content.`

	postPath := filepath.Join(tmpDir, "2023-12-25-test-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	// Verify post metadata
	if post.Title != "Test Post" {
		t.Errorf("Expected title 'Test Post', got '%s'", post.Title)
	}
	if post.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", post.Description)
	}
	if len(post.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(post.Tags))
	}
	if len(post.Categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(post.Categories))
	}

	// Verify that draft is set correctly
	if post.Draft {
		t.Error("Expected draft to be false")
	}

	// Verify slug generation
	if post.Slug != "test-post" {
		t.Errorf("Expected slug 'test-post', got '%s'", post.Slug)
	}

	// Verify URL generation
	if post.URL != "/test-post/" {
		t.Errorf("Expected URL '/test-post/', got '%s'", post.URL)
	}

	// Verify reading time calculation
	if post.ReadingTime == 0 {
		t.Error("Expected reading time to be > 0")
	}

	// Verify word count is calculated
	if post.WordCount == 0 {
		t.Error("Expected word count to be > 0")
	}
}

// TestParsePostWithoutSlug tests that slug is generated correctly from filename
func TestParsePostWithoutSlug(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Another Post"
date: 2023-12-26T10:00:00+08:00
draft: false
---

Content here.`

	// Test without slug in frontmatter
	postPath := filepath.Join(tmpDir, "2023-12-26-another-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if post.Slug != "another-post" {
		t.Errorf("Expected slug 'another-post', got '%s'", post.Slug)
	}
}

func TestParsePost_DateFallbackFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Filename Date"
draft: false
---

Content here.`

	postPath := filepath.Join(tmpDir, "2023-12-26-filename-date.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if post.Date.IsZero() {
		t.Fatal("Expected date to fall back from filename")
	}
	if got := post.Date.Format("2006-01-02"); got != "2023-12-26" {
		t.Fatalf("Expected fallback date 2023-12-26, got %s", got)
	}
}

// TestParsePostWithCustomSlug tests that custom slug in frontmatter is used
func TestParsePostWithCustomSlug(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Yet Another Post"
slug: "custom-slug"
date: 2023-12-27T10:00:00+08:00
draft: false
---

Content here.`

	postPath := filepath.Join(tmpDir, "2023-12-27-yet-another-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if post.Slug != "custom-slug" {
		t.Errorf("Expected slug 'custom-slug', got '%s'", post.Slug)
	}
}

// TestParsePost_Draft tests that draft posts are parsed correctly
func TestParsePost_Draft(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Draft Post"
date: 2023-12-28T10:00:00+08:00
draft: true
---

Draft content.`

	postPath := filepath.Join(tmpDir, "2023-12-28-draft-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if !post.Draft {
		t.Error("Expected draft to be true")
	}
}

func TestParsePost_StringFrontMatterLists(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "String Lists"
date: 2023-12-28T10:00:00+08:00
tags: 其它
categories: 后端
keywords: golang, yaml
aliases: /legacy/string-lists/
---

Content.`

	postPath := filepath.Join(tmpDir, "2023-12-28-string-lists.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if len(post.Tags) != 1 || post.Tags[0] != "其它" {
		t.Fatalf("Expected scalar tags to parse as one-item list, got %#v", post.Tags)
	}
	if len(post.Categories) != 1 || post.Categories[0] != "后端" {
		t.Fatalf("Expected scalar categories to parse as one-item list, got %#v", post.Categories)
	}
	if len(post.Keywords) != 2 || post.Keywords[0] != "golang" || post.Keywords[1] != "yaml" {
		t.Fatalf("Expected comma-separated keywords to parse, got %#v", post.Keywords)
	}
	if len(post.Aliases) != 1 || post.Aliases[0] != "/legacy/string-lists/" {
		t.Fatalf("Expected scalar aliases to parse as one-item list, got %#v", post.Aliases)
	}
}

func TestParsePost_Unpublished(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Hidden Post"
date: 2023-12-29T10:00:00+08:00
published: false
---

Hidden content.`

	postPath := filepath.Join(tmpDir, "2023-12-29-hidden-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	if post.Published == nil {
		t.Fatal("Expected published to be parsed")
	}
	if *post.Published {
		t.Error("Expected published to be false")
	}
}

func TestNormalizePostFrontMatter(t *testing.T) {
	raw := postFrontMatter{
		Title:       "Normalized Post",
		Date:        time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
		Description: "desc",
		Tags:        stringListNode("go"),
		Categories:  stringListNode("tech"),
		Layout:      "",
	}

	post, err := normalizePostFrontMatter(raw, filepath.Join("content", "2026-04-23-normalized-post.md"), "Hello world", "<p>Hello world</p>")
	if err != nil {
		t.Fatalf("normalizePostFrontMatter failed: %v", err)
	}
	if post.Slug != "normalized-post" {
		t.Fatalf("Expected derived slug normalized-post, got %q", post.Slug)
	}
	if post.URL != "/normalized-post/" {
		t.Fatalf("Expected derived URL /normalized-post/, got %q", post.URL)
	}
	if post.Layout != "post" {
		t.Fatalf("Expected default layout post, got %q", post.Layout)
	}
}

func stringListNode(values ...string) yaml.Node {
	content := make([]*yaml.Node, 0, len(values))
	for _, value := range values {
		content = append(content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: value,
		})
	}
	return yaml.Node{
		Kind:    yaml.SequenceNode,
		Content: content,
	}
}

func TestNormalizePageFrontMatter(t *testing.T) {
	raw := pageFrontMatter{
		Title:       "",
		Description: "desc",
		Permalink:   "",
	}

	page, err := normalizePageFrontMatter(raw, filepath.Join("pages", "about.md"), "pages", "About", "<p>About</p>")
	if err != nil {
		t.Fatalf("normalizePageFrontMatter failed: %v", err)
	}
	if page.Slug != "about" {
		t.Fatalf("Expected derived slug about, got %q", page.Slug)
	}
	if page.Title != "about" {
		t.Fatalf("Expected empty title to fall back to slug, got %q", page.Title)
	}
	if page.URL != "/about/" {
		t.Fatalf("Expected derived URL /about/, got %q", page.URL)
	}
	if page.Layout != "page" {
		t.Fatalf("Expected default layout page, got %q", page.Layout)
	}
}

// TestParsePost_MissingFrontMatter tests error handling for missing frontmatter
func TestParsePost_MissingFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `# Post Without FrontMatter

This post has no front matter.`

	postPath := filepath.Join(tmpDir, "no-frontmatter.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := ParsePost(postPath)
	if err == nil {
		t.Error("Expected error for missing frontmatter, got nil")
	}
}

// TestParsePosts tests parsing multiple posts
func TestParsePosts(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "nested"), 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create multiple test posts
	post1 := `---
title: "Post One"
date: 2023-12-25T10:00:00+08:00
draft: false
---

Content one.`

	post2 := `---
title: "Post Two"
date: 2023-12-26T10:00:00+08:00
draft: false
---

Content two.`

	post3 := `---
title: "Draft Post"
date: 2023-12-27T10:00:00+08:00
draft: true
---

Draft content.`

	if err := os.WriteFile(filepath.Join(tmpDir, "2023-12-25-post-one.md"), []byte(post1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "2023-12-26-post-two.md"), []byte(post2), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "nested", "2023-12-27-draft-post.markdown"), []byte(post3), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	posts, err := ParsePosts(tmpDir)
	if err != nil {
		t.Fatalf("ParsePosts failed: %v", err)
	}

	// Should parse all posts including drafts
	if len(posts) != 3 {
		t.Errorf("Expected 3 posts, got %d", len(posts))
	}
}

func TestParsePages(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	pageOne := `---
title: "About"
permalink: "/about/"
---

<div class="hero">hello</div>`

	pageTwo := `---
title: "Nested Page"
description: "Nested description"
---

# Nested`

	if err := os.WriteFile(filepath.Join(tmpDir, "about.md"), []byte(pageOne), 0644); err != nil {
		t.Fatalf("Failed to create page one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "docs", "nested.md"), []byte(pageTwo), 0644); err != nil {
		t.Fatalf("Failed to create page two: %v", err)
	}

	pages, err := ParsePages(tmpDir)
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("Expected 2 pages, got %d", len(pages))
	}

	if pages[0].ContentHTML == "" && pages[1].ContentHTML == "" {
		t.Fatal("Expected rendered HTML content for pages")
	}

	var foundAbout, foundNested bool
	for _, page := range pages {
		switch page.Title {
		case "About":
			foundAbout = true
			if page.URL != "/about/" {
				t.Fatalf("Expected /about/ URL, got %s", page.URL)
			}
			if !strings.Contains(page.ContentHTML, `<div class="hero">hello</div>`) {
				t.Fatalf("Expected raw HTML to be preserved, got %s", page.ContentHTML)
			}
		case "Nested Page":
			foundNested = true
			if page.URL != "/docs/nested/" {
				t.Fatalf("Expected nested URL, got %s", page.URL)
			}
		}
	}

	if !foundAbout || !foundNested {
		t.Fatalf("Expected both pages to be parsed, got %#v", pages)
	}
}

func TestParsePageWithOptions_DisablesUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	pagePath := filepath.Join(tmpDir, "unsafe.md")
	content := `---
title: "Unsafe"
---

<script>alert("x")</script>
<div class="hero">hello</div>`
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	page, err := ParsePageWithOptions(pagePath, tmpDir, RenderOptions{AllowUnsafeHTML: false})
	if err != nil {
		t.Fatalf("ParsePageWithOptions failed: %v", err)
	}
	if strings.Contains(page.ContentHTML, "<script>") || strings.Contains(page.ContentHTML, `<div class="hero">`) {
		t.Fatalf("Expected raw HTML to be disabled, got %s", page.ContentHTML)
	}
}

func TestParsePage_DefaultPreservesUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	pagePath := filepath.Join(tmpDir, "unsafe.md")
	content := `---
title: "Unsafe"
---

<div class="hero">hello</div>`
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	page, err := ParsePage(pagePath, tmpDir)
	if err != nil {
		t.Fatalf("ParsePage failed: %v", err)
	}
	if !strings.Contains(page.ContentHTML, `<div class="hero">hello</div>`) {
		t.Fatalf("Expected default parser to preserve raw HTML, got %s", page.ContentHTML)
	}
}

// TestSplitFrontMatter tests frontmatter parsing logic
func TestSplitFrontMatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantFront   string
		wantContent string
		wantErr     bool
	}{
		{
			name: "valid frontmatter",
			content: `---
title: Test
date: 2023-12-01T10:00:00+08:00
---

# Content

Paragraph.`,
			wantFront:   "title: Test\ndate: 2023-12-01T10:00:00+08:00",
			wantContent: "# Content\n\nParagraph.",
			wantErr:     false,
		},
		{
			name:        "missing opening delimiter",
			content:     "No frontmatter here\n\nContent.",
			wantFront:   "",
			wantContent: "No frontmatter here\n\nContent.",
			wantErr:     true,
		},
		{
			name: "missing closing delimiter",
			content: `---
title: Test
date: 2023-12-01T10:00:00+08:00

# Content`,
			wantFront:   "",
			wantContent: "",
			wantErr:     true,
		},
		{
			name: "empty frontmatter",
			content: `---
---

Content here.`,
			wantFront:   "",
			wantContent: "Content here.",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			front, content, err := splitFrontMatter(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitFrontMatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.Contains(front, strings.TrimSpace(tt.wantFront)) {
				t.Errorf("Front matter mismatch. Got: %s, want: %s", front, tt.wantFront)
			}
			if !strings.Contains(strings.TrimSpace(content), strings.TrimSpace(tt.wantContent)) {
				t.Errorf("Content mismatch. Got: %s, want: %s", content, tt.wantContent)
			}
		})
	}
}

// TestWordCount tests word counting functionality
func TestWordCount(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"one two three", 3},
		{"", 0},
		{"single", 1},
		{"  multiple   spaces   here  ", 3},
		{"line\nbreaks\nbetween", 3},
	}

	for _, tt := range tests {
		got := wordCount(tt.content)
		if got != tt.want {
			t.Errorf("wordCount(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

// TestGenerateSummary tests summary generation
func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantMin int
		wantMax int
	}{
		{
			name:    "short content",
			content: "Short content.",
			wantMin: 1,
			wantMax: 200,
		},
		{
			name:    "long content",
			content: strings.Repeat("This is a test sentence. ", 20),
			wantMin: 200,
			wantMax: 205, // Allow for ellipsis
		},
		{
			name:    "first line",
			content: "First line\n\nSecond line.",
			wantMin: 10,
			wantMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := generateSummary(tt.content)
			if len(summary) < tt.wantMin || len(summary) > tt.wantMax {
				t.Errorf("Summary length = %d, want between %d and %d", len(summary), tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestGenerateSummaryHTML tests HTML summary generation
func TestGenerateSummaryHTML(t *testing.T) {
	// Create minimal HTML content
	html := `<p>First paragraph content.</p><p>Second paragraph content.</p>`
	summary := generateSummaryHTML(html)

	// Should extract first paragraph
	if !strings.Contains(summary, "<p>") || !strings.Contains(summary, "</p>") {
		t.Error("Summary should contain a paragraph tag")
	}

	if !strings.Contains(summary, "First paragraph content.") {
		t.Error("Summary should contain content from first paragraph")
	}

	if strings.Contains(summary, "Second paragraph") {
		t.Error("Summary should not contain content from second paragraph")
	}
}

// TestPostRendering tests that markdown is rendered to HTML correctly
func TestPostRendering(t *testing.T) {
	tmpDir := t.TempDir()
	postContent := `---
title: "Rendering Test"
date: 2023-12-29T10:00:00+08:00
draft: false
---

# Heading 1

## Heading 2

This is a **bold** and *italic* text.

- List item 1
- List item 2

` + "```go\npackage main\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"

	postPath := filepath.Join(tmpDir, "2023-12-29-rendering.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	post, err := ParsePost(postPath)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}

	// Check HTML content
	if !strings.Contains(post.ContentHTML, "<h1>") {
		t.Error("Expected HTML to contain <h1> tag")
	}
	if !strings.Contains(post.ContentHTML, "<strong>") {
		t.Error("Expected HTML to contain <strong> tag for bold text")
	}
	if !strings.Contains(post.ContentHTML, "<em>") {
		t.Error("Expected HTML to contain <em> tag for italic text")
	}
	if !strings.Contains(post.ContentHTML, "<ul>") {
		t.Error("Expected HTML to contain <ul> tag for list")
	}
	if !strings.Contains(post.ContentHTML, "<pre>") {
		t.Error("Expected HTML to contain <pre> tag for code block")
	}
}

// BenchmarkParsePost benchmarks parsing a single post
func BenchmarkParsePost(b *testing.B) {
	tmpDir := b.TempDir()
	postContent := `---
title: "Benchmark Post"
date: 2023-12-25T10:00:00+08:00
description: "Benchmark description"
tags: ["benchmark", "test"]
categories: ["benchmarks"]
draft: false
---

# Benchmark Content

` + strings.Repeat("This is some test content. ", 100)

	postPath := filepath.Join(tmpDir, "2023-12-25-benchmark-post.md")
	if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParsePost(postPath)
		if err != nil {
			b.Fatalf("ParsePost failed: %v", err)
		}
	}
}

// BenchmarkParsePosts benchmarks parsing multiple posts
func BenchmarkParsePosts(b *testing.B) {
	tmpDir := b.TempDir()

	// Create multiple posts
	for i := 0; i < 10; i++ {
		postContent := fmt.Sprintf(`---
title: "Post %d"
date: 2023-12-%dT10:00:00+08:00
draft: false
---

Content %d.
`, i+1, i+1, i+1)

		postPath := filepath.Join(tmpDir, fmt.Sprintf("2023-12-%d-post-%d.md", i+10, i+1))
		if err := os.WriteFile(postPath, []byte(postContent), 0644); err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParsePosts(tmpDir)
		if err != nil {
			b.Fatalf("ParsePosts failed: %v", err)
		}
	}
}
