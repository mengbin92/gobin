package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mengbin92/blog/internal/config"
	"github.com/mengbin92/blog/internal/parser"
)

// SiteData represents data passed to templates
type SiteData struct {
	Site      *config.Config
	Title     string
	TitleSuffix string
	Canonical string
	Content   interface{}
}

// Generate generates the static site
func Generate(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	// Sort posts by date (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Load templates
	tmpl, err := loadTemplates()
	if err != nil {
		return err
	}

	// Generate index page (list of all posts)
	if err := generateIndex(posts, cfg, outputDir, tmpl); err != nil {
		return err
	}

	// Generate individual post pages
	for _, post := range posts {
		if err := generatePost(post, cfg, outputDir, tmpl); err != nil {
			return err
		}
	}

	// Copy static assets
	if err := copyStaticAssets(cfg.StaticDir, outputDir); err != nil {
		return err
	}

	return nil
}

// generateIndex generates the index page with post list
func generateIndex(posts []*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	// Apply pagination
	paginatedPosts := paginate(posts, cfg.Paginate)

	// Generate first page as index.html
	data := struct {
		Site      *config.Config
		Posts     []*parser.Post
		Title     string
	}{
		Site:  cfg,
		Posts: paginatedPosts[0],
		Title: cfg.Title,
	}

	return renderTemplate(tmpl, "listPage", filepath.Join(outputDir, "index.html"), data)
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
func loadTemplates() (*template.Template, error) {
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
	}

	tmpl := template.New("").Funcs(funcMap)

	// Parse base template first
	baseTmpl, err := tmpl.ParseFiles("templates/_default/base.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse base.html: %w", err)
	}

	// Parse other templates
	baseTmpl, err = baseTmpl.ParseFiles(
		"templates/_default/list.html",
		"templates/_default/single.html",
		"templates/_default/404.html",
		"templates/partials/header.html",
		"templates/partials/footer.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return baseTmpl, nil
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
func copyStaticAssets(staticDir, outputDir string) error {
	if staticDir == "" {
		staticDir = "assets"
	}

	return filepath.Walk(staticDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(staticDir, path)
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
