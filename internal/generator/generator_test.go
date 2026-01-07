package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// TestPaginate tests the pagination function
func TestPaginate(t *testing.T) {
	// Create test posts
	posts := []*parser.Post{
		{Title: "Post 1", Date: time.Now().Add(-time.Hour * 24)},
		{Title: "Post 2", Date: time.Now().Add(-time.Hour * 48)},
		{Title: "Post 3", Date: time.Now().Add(-time.Hour * 72)},
		{Title: "Post 4", Date: time.Now().Add(-time.Hour * 96)},
		{Title: "Post 5", Date: time.Now().Add(-time.Hour * 120)},
	}

	tests := []struct {
		name     string
		perPage  int
		wantPages int
		wantLastPageItems int
	}{
		{"5 posts per page", 5, 1, 5},
		{"3 posts per page", 3, 2, 2},
		{"2 posts per page", 2, 3, 1},
		{"1 post per page", 1, 5, 1},
		{"10 posts per page", 10, 1, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := paginate(posts, tt.perPage)

			// Count total items
			totalItems := 0
			for _, page := range pages {
				totalItems += len(page)
			}

			if len(pages) != tt.wantPages {
				t.Errorf("Expected %d pages, got %d", tt.wantPages, len(pages))
			}

			if totalItems != len(posts) {
				t.Errorf("Expected total items %d, got %d", len(posts), totalItems)
			}

			// Check last page size
			if len(pages) > 0 {
				lastPageSize := len(pages[len(pages)-1])
				if lastPageSize != tt.wantLastPageItems {
					t.Errorf("Expected last page to have %d items, got %d", tt.wantLastPageItems, lastPageSize)
				}
			}
		})
	}
}

// TestPaginate_Empty tests pagination with empty posts
func TestPaginate_Empty(t *testing.T) {
	pages := paginate([]*parser.Post{}, 10)

	if len(pages) != 0 {
		t.Errorf("Expected 0 pages for empty posts, got %d", len(pages))
	}
}

// TestPaginate_ZeroPerPage tests pagination with zero per page
func TestPaginate_ZeroPerPage(t *testing.T) {
	posts := []*parser.Post{
		{Title: "Post 1", Date: time.Now()},
		{Title: "Post 2", Date: time.Now().Add(-time.Hour)},
	}

	pages := paginate(posts, 0)

	// Should default to 10
	if len(pages) != 1 {
		t.Errorf("Expected 1 page with default page size, got %d", len(pages))
	}
}

// TestCopyFile tests file copying functionality
func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	srcContent := "Test content"
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create destination path
	dstFile := filepath.Join(tmpDir, "destination.txt")

	// Copy file
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination file exists and has correct content
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != srcContent {
		t.Errorf("Expected content '%s', got '%s'", srcContent, string(dstContent))
	}
}

// TestCopyFile_NonExistent tests error handling for non-existent source file
func TestCopyFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	dstFile := filepath.Join(tmpDir, "destination.txt")

	err := copyFile("/nonexistent/file.txt", dstFile)
	if err == nil {
		t.Error("Expected error for non-existent source file")
	}
}

// TestCopyStaticAssetsFromDir tests static asset copying with nested directories
func TestCopyStaticAssetsFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	assetsDir := filepath.Join(tmpDir, "assets")
	cssDir := filepath.Join(assetsDir, "css")
	jsDir := filepath.Join(assetsDir, "js")

	os.MkdirAll(cssDir, 0755)
	os.MkdirAll(jsDir, 0755)

	// Create test files
	cssFile := filepath.Join(cssDir, "style.css")
	jsFile := filepath.Join(jsDir, "script.js")
	rootFile := filepath.Join(assetsDir, "root.txt")

	if err := os.WriteFile(cssFile, []byte("body { margin: 0; }"), 0644); err != nil {
		t.Fatalf("Failed to create CSS file: %v", err)
	}
	if err := os.WriteFile(jsFile, []byte("console.log('test');"), 0644); err != nil {
		t.Fatalf("Failed to create JS file: %v", err)
	}
	if err := os.WriteFile(rootFile, []byte("root content"), 0644); err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}

	// Create output directory
	outputDir := filepath.Join(tmpDir, "output")

	// Copy static assets
	if err := copyStaticAssetsFromDir(assetsDir, outputDir, assetsDir); err != nil {
		t.Fatalf("copyStaticAssetsFromDir failed: %v", err)
	}

	// Verify files were copied
	verifyFiles := []string{
		filepath.Join(outputDir, "css", "style.css"),
		filepath.Join(outputDir, "js", "script.js"),
		filepath.Join(outputDir, "root.txt"),
	}

	for _, file := range verifyFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file to exist: %s", file)
		}
	}
}

// TestLoadTemplates tests template loading
func TestLoadTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create templates directory structure
	templatesDir := filepath.Join(tmpDir, "templates")
	defaultDir := filepath.Join(templatesDir, "_default")
	partialsDir := filepath.Join(templatesDir, "partials")

	os.MkdirAll(defaultDir, 0755)
	os.MkdirAll(partialsDir, 0755)

	// Create template files
	singleTemplate := `{{ define "singlePage" }}
<!DOCTYPE html>
<html>
<head><title>{{ .Title }}</title></head>
<body>{{ .Content }}</body>
</html>
{{ end }}`

	listTemplate := `{{ define "listPage" }}
<!DOCTYPE html>
<html>
<head><title>{{ .Title }}</title></head>
<body><ul>{{ range .Posts }}<li>{{ .Title }}</li>{{ end }}</ul></body>
</html>
{{ end }}`

	headerTemplate := `{{ define "header" }}
<header>{{ .Site.Title }}</header>
{{ end }}`

	footerTemplate := `{{ define "footer" }}
<footer>Footer</footer>
{{ end }}`

	template404 := `{{ define "404Page" }}
<!DOCTYPE html>
<html>
<head><title>404</title></head>
<body>Not Found</body>
</html>
{{ end }}`

	os.WriteFile(filepath.Join(defaultDir, "single.html"), []byte(singleTemplate), 0644)
	os.WriteFile(filepath.Join(defaultDir, "list.html"), []byte(listTemplate), 0644)
	os.WriteFile(filepath.Join(defaultDir, "404.html"), []byte(template404), 0644)
	os.WriteFile(filepath.Join(partialsDir, "header.html"), []byte(headerTemplate), 0644)
	os.WriteFile(filepath.Join(partialsDir, "footer.html"), []byte(footerTemplate), 0644)

	// Change to tmpDir temporarily to load templates
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Theme: "",
		StaticDir: "assets",
		PublishDir: "public",
		ThemesDir: "themes",
	}

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	if tmpl == nil {
		t.Error("Expected template to be loaded")
	}
}

// TestLoadTemplates_NoTemplates tests error handling when no templates exist
func TestLoadTemplates_NoTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to tmpDir temporarily
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Theme: "",
		StaticDir: "assets",
		PublishDir: "public",
		ThemesDir: "themes",
	}

	_, err := loadTemplates(cfg)
	if err == nil {
		t.Error("Expected error when no templates are found")
	}
}

// TestRenderTemplate tests template rendering
func TestRenderTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Create a simple template
	tmplContent := `{{ define "test" }}<!DOCTYPE html>
<html>
<head><title>{{ .Title }}</title></head>
<body>{{ .Content }}</body>
</html>
{{ end }}`

	tpl, err := template.New("test").Parse(tmplContent)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	outputFile := filepath.Join(outputDir, "test.html")
	data := struct {
		Title   string
		Content string
	}{
		Title:   "Test Title",
		Content: "Test Content",
	}

	if err := renderTemplate(tpl, "test", outputFile, data); err != nil {
		t.Fatalf("renderTemplate failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "Test Title") {
		t.Error("Expected output to contain title")
	}
	if !strings.Contains(string(content), "Test Content") {
		t.Error("Expected output to contain content")
	}
}

// TestGeneratePost tests individual post page generation
func TestGeneratePost(t *testing.T) {
	tmpDir := t.TempDir()

	// Create output directory
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates", "_default")
	os.MkdirAll(templatesDir, 0755)

	singleTemplate := `{{ define "singlePage" }}
<!DOCTYPE html>
<html>
<head><title>{{ .Title }}</title></head>
<body>
	<article>
		<h1>{{ .Post.Title }}</h1>
		<div class="content">{{ .Post.ContentHTML | safeHTML }}</div>
	</article>
</body>
</html>
{{ end }}`

	os.WriteFile(filepath.Join(templatesDir, "single.html"), []byte(singleTemplate), 0644)

	// Create post
	post := &parser.Post{
		Title:       "Test Post",
		Date:        time.Now(),
		Slug:        "test-post",
		URL:         "/test-post/",
		Content:     "Test content",
		ContentHTML: "<p>Test content</p>",
	}

	// Change to tmpDir
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Title: "Test Blog",
	}

	// Load template
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	// Generate post
	if err := generatePost(post, cfg, outputDir, tmpl); err != nil {
		t.Fatalf("generatePost failed: %v", err)
	}

	// Verify output file
	postFile := filepath.Join(outputDir, "test-post", "index.html")
	if _, err := os.Stat(postFile); os.IsNotExist(err) {
		t.Fatal("Expected post file to be created")
	}
}

// BenchmarkPaginate benchmarks pagination function
func BenchmarkPaginate(b *testing.B) {
	// Create test posts
	posts := make([]*parser.Post, 100)
	for i := 0; i < 100; i++ {
		posts[i] = &parser.Post{
			Title: fmt.Sprintf("Post %d", i),
			Date:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = paginate(posts, 10)
	}
}

// BenchmarkCopyFile benchmarks file copying
func BenchmarkCopyFile(b *testing.B) {
	tmpDir := b.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	srcContent := strings.Repeat("Test content", 1000)
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		b.Fatalf("Failed to create source file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create new destination each iteration
		dstFile := filepath.Join(tmpDir, fmt.Sprintf("destination-%d.txt", i))
		if err := copyFile(srcFile, dstFile); err != nil {
			b.Fatalf("copyFile failed: %v", err)
		}
	}
}
