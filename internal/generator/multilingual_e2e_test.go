package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// writeMultilingualTemplates overlays a custom single-page template onto
// the repo template set: it renders the v1.9.0 multilingual page data
// (T strings, translation links, language switcher) so the end-to-end
// tests can assert on them.
func writeMultilingualTemplates(t *testing.T, siteDir string) {
	t.Helper()
	custom := `{{ define "singleMain" }}
<article>
<h1>{{ .Post.Title }}</h1>
<p class="t-string">{{ T "readMore" }}</p>
<nav class="lang-switch">{{ range .Languages }}{{ if .Active }}<span class="active" data-lang="{{ .Lang }}">{{ .Name }}</span>{{ else }}<a href="{{ .URL }}" data-lang="{{ .Lang }}">{{ .Name }}</a>{{ end }}{{ end }}</nav>
{{ if .Post.Translations }}<ul class="translations">{{ range .Post.Translations }}<li><a href="{{ .URL }}" hreflang="{{ .Name }}">{{ .Title }}</a></li>{{ end }}</ul>{{ end }}
{{ if .PrevPost }}<a class="prev" href="{{ url .PrevPost.URL }}">{{ .PrevPost.Title }}</a>{{ end }}
{{ if .NextPost }}<a class="next" href="{{ url .NextPost.URL }}">{{ .NextPost.Title }}</a>{{ end }}
</article>
{{ end }}
`
	path := filepath.Join(siteDir, "templates", "_default", "zz-multilingual.html")
	if err := os.WriteFile(path, []byte(custom), 0644); err != nil {
		t.Fatalf("write custom template: %v", err)
	}
}

func setupMultilingualSite(t *testing.T) (siteDir, outputDir string) {
	t.Helper()
	siteDir = t.TempDir()
	outputDir = filepath.Join(siteDir, "public")
	if err := createGoldenTestSite(siteDir); err != nil {
		t.Fatalf("create test site: %v", err)
	}
	writeMultilingualTemplates(t, siteDir)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })
	return siteDir, outputDir
}

func readOutputFile(t *testing.T, outputDir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestGenerateMultilingual_EndToEnd(t *testing.T) {
	_, outputDir := setupMultilingualSite(t)

	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	enHello := multilingualTestPost("Hello", "hello", "greeting", date)
	enSecond := multilingualTestPost("Second", "second", "", date.AddDate(0, 0, -1))
	zhHello := multilingualTestPost("你好", "hello-zh", "greeting", date)
	zhSecond := multilingualTestPost("第二篇", "second-zh", "", date.AddDate(0, 0, -1))

	cfg := multilingualTestConfig()
	_, err := GenerateMultilingualWithOptions(
		LanguageContent{Posts: []*parser.Post{enHello, enSecond}},
		map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhHello, zhSecond}}},
		cfg,
		GenerationOptions{OutputDir: outputDir, CleanOutput: true},
	)
	if err != nil {
		t.Fatalf("GenerateMultilingualWithOptions failed: %v", err)
	}

	// Default language lives at the root, zh under /zh/.
	for _, rel := range []string{
		"index.html", "hello/index.html", "index.xml", "index.atom",
		"sitemap.xml", "search-index.json", "404.html", "robots.txt",
		".gobin-build.json",
		"zh/index.html", "zh/hello-zh/index.html", "zh/second-zh/index.html",
		"zh/index.xml", "zh/index.atom", "zh/sitemap.xml",
		"zh/search-index.json", "zh/404.html", "zh/.gobin-build.json",
		"zh/tags/index.html", "zh/categories/index.html",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected output %s: %v", rel, err)
		}
	}
	// robots.txt and aliases are root-only.
	if _, err := os.Stat(filepath.Join(outputDir, "zh", "robots.txt")); !os.IsNotExist(err) {
		t.Error("zh/robots.txt should not exist")
	}

	enPage := readOutputFile(t, outputDir, "hello/index.html")
	zhPage := readOutputFile(t, outputDir, "zh/hello-zh/index.html")

	// <html lang> follows the per-language languageCode.
	if !strings.Contains(enPage, `<html lang="en"`) {
		t.Error("en page missing lang=\"en\"")
	}
	if !strings.Contains(zhPage, `<html lang="zh-CN"`) {
		t.Error("zh page missing lang=\"zh-CN\"")
	}

	// T renders the per-language string table.
	if !strings.Contains(enPage, `<p class="t-string">Read more</p>`) {
		t.Error("en page missing default-language T string")
	}
	if !strings.Contains(zhPage, `<p class="t-string">阅读更多</p>`) {
		t.Error("zh page missing translated T string")
	}

	// Translation links cross-reference the pair with prefixed URLs.
	if !strings.Contains(enPage, `href="/zh/hello-zh/" hreflang="zh-CN">你好</a>`) {
		t.Error("en page missing link to zh translation")
	}
	if !strings.Contains(zhPage, `href="/hello/" hreflang="en">Hello</a>`) {
		t.Error("zh page missing link to en translation")
	}

	// Canonical URLs carry the language prefix.
	if !strings.Contains(zhPage, `href="https://example.com/zh/hello-zh/"`) {
		t.Error("zh page canonical missing /zh/ prefix")
	}

	// Language switcher: active language is a span, the other a link.
	if !strings.Contains(zhPage, `<span class="active" data-lang="zh">zh-CN</span>`) ||
		!strings.Contains(zhPage, `<a href="/" data-lang="">en</a>`) {
		t.Error("zh page language switcher wrong")
	}

	// prev/next stay within the language: the zh second post links only
	// to the zh first post, never to en titles.
	zhSecondPage := readOutputFile(t, outputDir, "zh/second-zh/index.html")
	if !strings.Contains(zhSecondPage, `class="next" href="/zh/hello-zh/"`) {
		t.Error("zh second post should link to zh hello post")
	}
	if strings.Contains(zhSecondPage, ">Hello</a>") || strings.Contains(zhSecondPage, ">Second</a>") {
		t.Error("zh page leaked en prev/next link")
	}

	// The zh list page contains only zh posts.
	zhIndex := readOutputFile(t, outputDir, "zh/index.html")
	if !strings.Contains(zhIndex, "第二篇") || strings.Contains(zhIndex, ">Hello<") {
		t.Error("zh index should list only zh posts")
	}

	// Feeds and sitemap carry language-prefixed absolute URLs.
	zhFeed := readOutputFile(t, outputDir, "zh/index.xml")
	if !strings.Contains(zhFeed, "https://example.com/zh/hello-zh/") {
		t.Error("zh feed missing prefixed URLs")
	}
	zhSitemap := readOutputFile(t, outputDir, "zh/sitemap.xml")
	if !strings.Contains(zhSitemap, "https://example.com/zh/hello-zh/") {
		t.Error("zh sitemap missing prefixed URLs")
	}

	// Per-language search index links into its own subtree.
	zhSearch := readOutputFile(t, outputDir, "zh/search-index.json")
	if !strings.Contains(zhSearch, `"/zh/hello-zh/"`) {
		t.Error("zh search index missing prefixed URLs")
	}

	// Static assets are duplicated per language (documented trade-off).
	if _, err := os.Stat(filepath.Join(outputDir, "zh", "css", "main.css")); err != nil {
		t.Error("zh subtree should carry its own static assets")
	}
}

// writeRealPost writes a markdown post file and parses it, so the
// incremental build manifest has real source files to hash.
func writeRealPost(t *testing.T, dir, name string, frontMatter map[string]string, body string) *parser.Post {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	for k, v := range frontMatter {
		fmt.Fprintf(&sb, "%s: %s\n", k, v)
	}
	sb.WriteString("---\n")
	sb.WriteString(body)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
	post, err := parser.ParsePost(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return post
}

func TestGenerateMultilingual_Incremental(t *testing.T) {
	siteDir, outputDir := setupMultilingualSite(t)
	cfg := multilingualTestConfig()

	enDir := filepath.Join(siteDir, "_posts")
	zhDir := filepath.Join(siteDir, "_posts", "zh")
	enPost := writeRealPost(t, enDir, "2026-03-20-hello.md",
		map[string]string{"title": "Hello", "translationKey": "greeting"}, "en body")
	zhPost := writeRealPost(t, zhDir, "2026-03-20-hello.md",
		map[string]string{"title": "你好", "slug": "hello-zh", "translationKey": "greeting"}, "zh body")

	build := func(en, zh *parser.Post, clean bool) *GenerationResult {
		t.Helper()
		result, err := GenerateMultilingualWithOptions(
			LanguageContent{Posts: []*parser.Post{en}},
			map[string]LanguageContent{"zh": {Posts: []*parser.Post{zh}}},
			cfg,
			GenerationOptions{OutputDir: outputDir, CleanOutput: clean, Incremental: !clean},
		)
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}
		return result
	}

	build(enPost, zhPost, true)

	// Second build with identical inputs: everything skips.
	second := build(enPost, zhPost, false)
	if second.Pages.Rendered != 0 {
		t.Errorf("second build rendered %d pages, want 0 (all skipped)", second.Pages.Rendered)
	}

	// Change the zh post's slug: the en post that links to it must
	// re-render too, even though its own source bytes are unchanged.
	zhMoved := writeRealPost(t, zhDir, "2026-03-20-hello.md",
		map[string]string{"title": "你好", "slug": "moved-zh", "translationKey": "greeting"}, "zh body")
	third := build(enPost, zhMoved, false)
	if third.Pages.Rendered < 2 {
		t.Errorf("third build rendered %d pages, want >= 2 (zh post + linked en post)", third.Pages.Rendered)
	}
	enPage := readOutputFile(t, outputDir, "hello/index.html")
	if !strings.Contains(enPage, `href="/zh/moved-zh/"`) {
		t.Error("en post should re-render with the updated translation URL")
	}
}

func TestGenerateMultilingual_MonolingualFallback(t *testing.T) {
	// Without languages: the multilingual entry point must behave
	// exactly like GenerateWithOptions.
	_, outputDir := setupMultilingualSite(t)
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	post := multilingualTestPost("Hello", "hello", "", date)

	cfg := &config.Config{
		Title: "Mono", LanguageCode: "en", BaseURL: "https://example.com",
		StaticDir: "assets", Paginate: 10, PaginatePath: "page",
	}
	_, err := GenerateMultilingualWithOptions(
		LanguageContent{Posts: []*parser.Post{post}}, nil, cfg,
		GenerationOptions{OutputDir: outputDir, CleanOutput: true},
	)
	if err != nil {
		t.Fatalf("monolingual fallback failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "hello", "index.html")); err != nil {
		t.Error("monolingual fallback should render at the root")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "zh")); !os.IsNotExist(err) {
		t.Error("monolingual fallback should not create language subtrees")
	}
}

// TestGenerate_MultilingualSiteGolden snapshots a full bilingual build
// (en default at root + zh under /zh/) against the golden fixture tree.
func TestGenerate_MultilingualSiteGolden(t *testing.T) {
	// findRepoRoot must run before setupMultilingualSite chdirs into the
	// temp site (which itself carries templates/ + assets/).
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to locate repository root: %v", err)
	}
	_, outputDir := setupMultilingualSite(t)

	enAlpha := &parser.Post{
		Title: "Alpha Post", Slug: "alpha-post",
		Date:        time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		Description: "Alpha description.", Summary: "Alpha summary.",
		SummaryHTML: "<p>Alpha summary.</p>",
		Content:     "Alpha body content.", ContentHTML: "<p>Alpha body content.</p>",
		Tags: []string{"Go"}, Categories: []string{"Tech"},
		TranslationKey: "alpha", ReadingTime: 1,
	}
	enBeta := &parser.Post{
		Title: "Beta Post", Slug: "beta-post",
		Date:        time.Date(2026, 3, 18, 9, 30, 0, 0, time.UTC),
		Description: "Beta description.", Summary: "Beta summary.",
		SummaryHTML: "<p>Beta summary.</p>",
		Content:     "Beta body content.", ContentHTML: "<p>Beta body content.</p>",
		Tags: []string{"Go"}, Categories: []string{"Tech"},
		Aliases: []string{"/old-beta/"}, ReadingTime: 1,
	}
	zhAlpha := &parser.Post{
		Title: "阿尔法", Slug: "alpha-post-zh",
		Date:        time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		Description: "阿尔法简介。", Summary: "阿尔法摘要。",
		SummaryHTML: "<p>阿尔法摘要。</p>",
		Content:     "阿尔法正文。", ContentHTML: "<p>阿尔法正文。</p>",
		Tags: []string{"Go"}, Categories: []string{"Tech"},
		TranslationKey: "alpha", ReadingTime: 1,
	}

	cfg := &config.Config{
		Title:           "Golden Blog",
		Description:     "Golden site description.",
		Author:          "Golden Author",
		LanguageCode:    "en-us",
		BaseURL:         "https://example.com",
		StaticDir:       "assets",
		ThemesDir:       "themes",
		Paginate:        1,
		PaginatePath:    "page",
		EnableRobotsTXT: true,
		Permalinks: map[string]string{
			"posts": "/:year/:month/:day/:slug/",
		},
		Strings: map[string]string{"readMore": "Read more"},
		Languages: map[string]*config.LanguageConfig{
			"zh": {
				LanguageCode: "zh-CN",
				Title:        "金色博客",
				Strings:      map[string]string{"readMore": "阅读更多"},
			},
		},
	}

	_, err = GenerateMultilingualWithOptions(
		LanguageContent{Posts: []*parser.Post{enAlpha, enBeta}},
		map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhAlpha}}},
		cfg,
		GenerationOptions{OutputDir: outputDir, CleanOutput: true},
	)
	if err != nil {
		t.Fatalf("GenerateMultilingualWithOptions failed: %v", err)
	}

	expectedFiles := multilingualGoldenExpectedFiles()
	assertGoldenSiteOutput(t, repoRoot, outputDir, "multilingual_site", expectedFiles)
}

func multilingualGoldenExpectedFiles() []string {
	files := []string{
		".gobin-assets.json",
		".gobin-build.json",
		"2026/03/18/beta-post/index.html",
		"2026/03/20/alpha-post/index.html",
		"404.html",
		"categories/index.html",
		"categories/tech/index.html",
		"css/main.css",
		"index.atom",
		"index.html",
		"index.xml",
		"old-beta/index.html",
		"page/2/index.html",
		"robots.txt",
		"search-index-min.json",
		"search-index.json",
		"sitemap.xml",
		"tags/go/index.html",
		"tags/index.html",
	}
	for _, rel := range []string{
		".gobin-assets.json",
		".gobin-build.json",
		"2026/03/20/alpha-post-zh/index.html",
		"404.html",
		"categories/index.html",
		"categories/tech/index.html",
		"css/main.css",
		"index.atom",
		"index.html",
		"index.xml",
		"search-index-min.json",
		"search-index.json",
		"sitemap.xml",
		"tags/go/index.html",
		"tags/index.html",
	} {
		files = append(files, "zh/"+rel)
	}
	return files
}
