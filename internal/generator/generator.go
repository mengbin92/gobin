package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// SiteData represents data passed to templates
type SiteData struct {
	Site      *config.Config
	Title     string
	TitleSuffix string
	Canonical string
	Content   interface{}
}

// Pagination represents pagination information for templates
type Pagination struct {
	Page        int
	TotalPages  int
	TotalPosts  int
	PrevPage    int
	NextPage    int
	IsFirstPage bool
	IsLastPage  bool
}

// Generate generates the static site
func Generate(posts []*parser.Post, cfg *config.Config, outputDir string, minify bool) error {
	// Sort posts by date (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Load templates
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		return err
	}

	// Generate index page (list of all posts with pagination)
	if err := generateIndex(posts, cfg, outputDir, tmpl); err != nil {
		return err
	}

	// Generate individual post pages
	for _, post := range posts {
		if err := generatePost(post, cfg, outputDir, tmpl); err != nil {
			return err
		}
	}

	// Generate taxonomy pages (tags and categories)
	tags, categories, err := generateTaxonomyPages(posts, cfg, outputDir, tmpl)
	if err != nil {
		return err
	}

	// Generate RSS and Atom feeds
	if err := GenerateFeeds(posts, cfg, outputDir); err != nil {
		return err
	}

	// Generate sitemap
	if err := GenerateSitemap(posts, cfg, outputDir, tags, categories); err != nil {
		return err
	}

	// Generate search indexes
	if err := GenerateSearch(posts, cfg, outputDir); err != nil {
		return err
	}

	// Copy static assets
	if err := copyStaticAssets(cfg, outputDir); err != nil {
		return err
	}

	// Minify output if requested
	if minify {
		if err := minifyOutput(outputDir); err != nil {
			return err
		}
	}

	return nil
}

// generateIndex generates the index page with post list
func generateIndex(posts []*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	// Apply pagination
	paginatedPosts := paginate(posts, cfg.Paginate)

	// Generate pagination pages
	for i, pagePosts := range paginatedPosts {
		pageNum := i + 1
		isFirstPage := pageNum == 1
		isLastPage := pageNum == len(paginatedPosts)

		// Prepare pagination data
		paginationData := Pagination{
			Page:        pageNum,
			TotalPages:  len(paginatedPosts),
			TotalPosts:  len(posts),
			PrevPage:    0,
			NextPage:    0,
			IsFirstPage: isFirstPage,
			IsLastPage:  isLastPage,
		}

		if !isFirstPage {
			paginationData.PrevPage = pageNum - 1
		}
		if !isLastPage {
			paginationData.NextPage = pageNum + 1
		}

		// Prepare data for template
		data := struct {
			Site       *config.Config
			Posts      []*parser.Post
			Title      string
			Pagination Pagination
		}{
			Site:       cfg,
			Posts:      pagePosts,
			Title:      cfg.Title,
			Pagination: paginationData,
		}

		// Generate file path
		var filePath string
		if isFirstPage {
			filePath = filepath.Join(outputDir, "index.html")
		} else {
			// Create page/ directory for pagination
			pageDir := filepath.Join(outputDir, cfg.PaginatePath, fmt.Sprintf("%d", pageNum))
			if err := os.MkdirAll(pageDir, 0755); err != nil {
				return err
			}
			filePath = filepath.Join(pageDir, "index.html")
		}

		// Render template
		if err := renderTemplate(tmpl, "listPage", filePath, data); err != nil {
			return err
		}
	}

	return nil
}

// generatePost generates a single post page
func generatePost(post *parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	data := struct {
		Site      *config.Config
		Post      *parser.Post
		Title     string
	}{
		Site:  cfg,
		Post:  post,
		Title: post.Title + " - " + cfg.Title,
	}

	// Create subdirectory for the post based on its URL
	postDir := filepath.Join(outputDir, strings.TrimPrefix(strings.TrimSuffix(post.URL, "/"), "/"))
	if err := os.MkdirAll(postDir, 0755); err != nil {
		return err
	}

	return renderTemplate(tmpl, "singlePage", filepath.Join(postDir, "index.html"), data)
}

// loadTemplates loads HTML templates with custom functions
func loadTemplates(cfg *config.Config) (*template.Template, error) {
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"dateFormat": func(layout string, t time.Time) string {
			return t.Format(layout)
		},
		"now": func() time.Time {
			return time.Now()
		},
		"urlize": urlize,
		"first": func(n int, list interface{}) interface{} {
			switch v := list.(type) {
			case []string:
				if len(v) < n {
					return v
				}
				return v[:n]
			case []*parser.Post:
				if len(v) < n {
					return v
				}
				return v[:n]
			default:
				return nil
			}
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	// Get template directories based on theme configuration
	templatePaths := getTemplatePaths(cfg)

	// Parse templates
	var templateFiles []string
	for _, path := range templatePaths {
		if _, err := os.Stat(path); err == nil {
			templateFiles = append(templateFiles, path)
		}
	}

	if len(templateFiles) == 0 {
		return nil, fmt.Errorf("no templates found")
	}

	tmpl, err := tmpl.ParseFiles(templateFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return tmpl, nil
}

// getTemplatePaths returns template file paths based on theme configuration
func getTemplatePaths(cfg *config.Config) []string {
	var paths []string

	themesDir := cfg.ThemesDir
	if themesDir == "" {
		themesDir = "themes"
	}

	// Define all template files we need
	tmplFiles := []string{
		"_default/single.html",
		"_default/list.html",
		"_default/404.html",
		"partials/header.html",
		"partials/footer.html",
	}

	// If a theme is specified, try to load from theme directory first
	if cfg.Theme != "" {
		themeDir := filepath.Join(themesDir, cfg.Theme, "layouts")
		if _, err := os.Stat(themeDir); err == nil {
			for _, tmplFile := range tmplFiles {
				themeTmplPath := filepath.Join(themeDir, tmplFile)
				if _, err := os.Stat(themeTmplPath); err == nil {
					paths = append(paths, themeTmplPath)
				}
			}
		}
	}

	// Always add default templates as fallback (don't overwrite if already added from theme)
	defaultDir := "templates"
	for _, tmplFile := range tmplFiles {
		defaultTmplPath := filepath.Join(defaultDir, tmplFile)
		if _, err := os.Stat(defaultTmplPath); err == nil {
			// Check if we already added this from theme
			alreadyAdded := false
			for _, existingPath := range paths {
				if strings.HasSuffix(existingPath, tmplFile) {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				paths = append(paths, defaultTmplPath)
			}
		}
	}

	return paths
}

// renderTemplate renders a template to a file
func renderTemplate(tmpl *template.Template, name, path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, name, data)
}

// paginate splits posts into pages
func paginate(posts []*parser.Post, perPage int) [][]*parser.Post {
	if perPage <= 0 {
		perPage = 10
	}

	var pages [][]*parser.Post
	for i := 0; i < len(posts); i += perPage {
		end := i + perPage
		if end > len(posts) {
			end = len(posts)
		}
		pages = append(pages, posts[i:end])
	}
	return pages
}

// copyStaticAssets copies static files to output directory
func copyStaticAssets(cfg *config.Config, outputDir string) error {
	// Copy from theme static directory first (if theme is specified)
	if cfg.Theme != "" && cfg.ThemesDir != "" {
		themeStaticDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")
		if _, err := os.Stat(themeStaticDir); err == nil {
			if err := copyStaticAssetsFromDir(themeStaticDir, outputDir, themeStaticDir); err != nil {
				return err
			}
		}
	}

	// Copy from main static directory
	staticDir := cfg.StaticDir
	if staticDir == "" {
		staticDir = "assets"
	}

	return copyStaticAssetsFromDir(staticDir, outputDir, staticDir)
}

// copyStaticAssetsFromDir copies static files from a specific directory
func copyStaticAssetsFromDir(sourceDir, outputDir, baseDir string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Create destination path
		destPath := filepath.Join(outputDir, relPath)

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Copy file
		return copyFile(path, destPath)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// generateTaxonomyPages generates tag and category pages
func generateTaxonomyPages(posts []*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) ([]string, []string, error) {
	// Collect all tags and categories
	tagMap := make(map[string][]*parser.Post)
	categoryMap := make(map[string][]*parser.Post)

	for _, post := range posts {
		// Collect tags
		for _, tag := range post.Tags {
			tag = strings.ToLower(tag)
			tagMap[tag] = append(tagMap[tag], post)
		}

		// Collect categories
		for _, category := range post.Categories {
			category = strings.ToLower(category)
			categoryMap[category] = append(categoryMap[category], post)
		}
	}

	// Generate tag pages
	if err := generateTagPages(tagMap, cfg, outputDir, tmpl); err != nil {
		return nil, nil, err
	}

	// Generate category pages
	if err := generateCategoryPages(categoryMap, cfg, outputDir, tmpl); err != nil {
		return nil, nil, err
	}

	// Extract tag and category lists
	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	var categories []string
	for category := range categoryMap {
		categories = append(categories, category)
	}

	return tags, categories, nil
}

// TagPageData represents data for tag pages
type TagPageData struct {
	Tag        string
	Posts      []*parser.Post
	Site       *config.Config
	Title      string
}

// generateTagPages generates individual tag pages and tag index
func generateTagPages(tagMap map[string][]*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	// Generate tag index page (/tags/)
	tagsDir := filepath.Join(outputDir, "tags")
	if err := os.MkdirAll(tagsDir, 0755); err != nil {
		return err
	}

	// Sort tags alphabetically
	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	// For tag index, we'll create a simple HTML file directly since it has different structure
	tagIndexPath := filepath.Join(tagsDir, "index.html")
	f, err := os.Create(tagIndexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write simple tag index HTML
	tagIndexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tags - %s</title>
    <link rel="stylesheet" href="/css/main.css">
</head>
<body>
    <header>
        <h1>All Tags</h1>
    </header>
    <main>
        <ul class="tag-list">
`, cfg.LanguageCode, cfg.Title)

	for _, tag := range tags {
		tagIndexHTML += fmt.Sprintf(`            <li><a href="/tags/%s/">%s</a></li>
`, urlize(tag), tag)
	}

	tagIndexHTML += `        </ul>
    </main>
</body>
</html>`

	if _, err := f.WriteString(tagIndexHTML); err != nil {
		return err
	}

	// Generate individual tag pages
	for tag, posts := range tagMap {
		// Sort posts by date (newest first)
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].Date.After(posts[j].Date)
		})

		// Create tag directory
		tagDir := filepath.Join(outputDir, "tags", urlize(tag))
		if err := os.MkdirAll(tagDir, 0755); err != nil {
			return err
		}

		// For individual tag pages, create simple HTML with list of posts
		tagPagePath := filepath.Join(tagDir, "index.html")
		f, err := os.Create(tagPagePath)
		if err != nil {
			return err
		}

		pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tag: %s - %s</title>
    <link rel="stylesheet" href="/css/main.css">
</head>
<body>
    <header>
        <h1>Tag: %s</h1>
        <p><a href="/tags/">← All Tags</a></p>
    </header>
    <main>
        <div class="post-list">
`, cfg.LanguageCode, tag, cfg.Title, tag)

		for _, post := range posts {
			pageHTML += fmt.Sprintf(`            <article class="post-item">
                <h2><a href="%s">%s</a></h2>
                <div class="post-meta">
                    <time>%s</time>
                </div>
                <div class="post-summary">%s</div>
            </article>
`, post.URL, post.Title, post.Date.Format("2006-01-02"), post.Summary)
		}

		pageHTML += `        </div>
    </main>
</body>
</html>`

		if _, err := f.WriteString(pageHTML); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// generateCategoryPages generates individual category pages and category index
func generateCategoryPages(categoryMap map[string][]*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	// Generate category index page (/categories/)
	categoriesDir := filepath.Join(outputDir, "categories")
	if err := os.MkdirAll(categoriesDir, 0755); err != nil {
		return err
	}

	// Sort categories alphabetically
	var categories []string
	for category := range categoryMap {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	// For category index, we'll create a simple HTML file directly
	categoryIndexPath := filepath.Join(categoriesDir, "index.html")
	f, err := os.Create(categoryIndexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write simple category index HTML
	categoryIndexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Categories - %s</title>
    <link rel="stylesheet" href="/css/main.css">
</head>
<body>
    <header>
        <h1>All Categories</h1>
    </header>
    <main>
        <ul class="category-list">
`, cfg.LanguageCode, cfg.Title)

	for _, category := range categories {
		categoryIndexHTML += fmt.Sprintf(`            <li><a href="/categories/%s/">%s</a></li>
`, urlize(category), category)
	}

	categoryIndexHTML += `        </ul>
    </main>
</body>
</html>`

	if _, err := f.WriteString(categoryIndexHTML); err != nil {
		return err
	}

	// Generate individual category pages
	for category, posts := range categoryMap {
		// Sort posts by date (newest first)
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].Date.After(posts[j].Date)
		})

		// Create category directory
		categoryDir := filepath.Join(outputDir, "categories", urlize(category))
		if err := os.MkdirAll(categoryDir, 0755); err != nil {
			return err
		}

		// For individual category pages, create simple HTML with list of posts
		categoryPagePath := filepath.Join(categoryDir, "index.html")
		f, err := os.Create(categoryPagePath)
		if err != nil {
			return err
		}

		pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Category: %s - %s</title>
    <link rel="stylesheet" href="/css/main.css">
</head>
<body>
    <header>
        <h1>Category: %s</h1>
        <p><a href="/categories/">← All Categories</a></p>
    </header>
    <main>
        <div class="post-list">
`, cfg.LanguageCode, category, cfg.Title, category)

		for _, post := range posts {
			pageHTML += fmt.Sprintf(`            <article class="post-item">
                <h2><a href="%s">%s</a></h2>
                <div class="post-meta">
                    <time>%s</time>
                </div>
                <div class="post-summary">%s</div>
            </article>
`, post.URL, post.Title, post.Date.Format("2006-01-02"), post.Summary)
		}

		pageHTML += `        </div>
    </main>
</body>
</html>`

		if _, err := f.WriteString(pageHTML); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// urlize converts a string to URL-friendly format
func urlize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Keep alphanumeric, Chinese characters, and dashes
	var result strings.Builder
	for _, r := range s {
		// Keep ASCII alphanumeric, dash, and Chinese characters
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			(r >= 0x4e00 && r <= 0x9fff) || // CJK Unified Ideographs
			(r >= 0x3400 && r <= 0x4dbf) || // CJK Extension A
			(r >= 0x20000 && r <= 0x2a6df) { // CJK Extension B
			result.WriteRune(r)
		}
	}

	// Remove consecutive dashes
	url := result.String()
	for strings.Contains(url, "--") {
		url = strings.ReplaceAll(url, "--", "-")
	}

	// Remove leading/trailing dashes
	url = strings.Trim(url, "-")

	// If resulting URL is empty, use a fallback
	if url == "" {
		return "untitled"
	}

	return url
}

// minifyOutput minifies HTML, CSS, and JS files in the output directory
func minifyOutput(outputDir string) error {
	return filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process HTML, CSS, and JS files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Minify content
		minified := minifyContent(string(content), ext)

		// Write back
		return os.WriteFile(path, []byte(minified), 0644)
	})
}

// minifyContent performs basic minification on content
func minifyContent(content string, ext string) string {
	// Remove comments
	content = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(content, "")

	// Remove extra whitespace
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`>\s+<`).ReplaceAllString(content, "><")
	content = regexp.MustCompile(`\s*{\s*`).ReplaceAllString(content, "{")
	content = regexp.MustCompile(`\s*}\s*`).ReplaceAllString(content, "}")
	content = regexp.MustCompile(`\s*;\s*`).ReplaceAllString(content, ";")
	content = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(content, ",")

	// Trim leading/trailing whitespace
	content = strings.TrimSpace(content)

	return content
}
