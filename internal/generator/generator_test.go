package generator

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func boolPtr(v bool) *bool {
	return &v
}

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
		name              string
		perPage           int
		wantPages         int
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

func TestBuildPostURL(t *testing.T) {
	post := &parser.Post{
		Title: "Hello Gobin",
		Slug:  "hello-gobin",
		Date:  time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}
	cfg := &config.Config{
		Permalinks: map[string]string{
			"posts": "/:year/:month/:day/:title/",
		},
	}

	got := buildPostURL(post, cfg)
	want := "/2026/03/28/hello-gobin/"
	if got != want {
		t.Fatalf("Expected URL %q, got %q", want, got)
	}
}

func TestPreparePostsVisibility(t *testing.T) {
	posts := []*parser.Post{
		{Title: "published", Slug: "published", Date: time.Now()},
		{Title: "draft", Slug: "draft", Date: time.Now().Add(-time.Hour), Draft: true},
		{Title: "hidden", Slug: "hidden", Date: time.Now().Add(-2 * time.Hour), Published: boolPtr(false)},
	}
	cfg := &config.Config{}

	visible := preparePosts(posts, cfg, false)
	if len(visible) != 1 {
		t.Fatalf("Expected 1 visible post by default, got %d", len(visible))
	}
	if visible[0].Slug != "published" {
		t.Fatalf("Expected published post to remain visible, got %q", visible[0].Slug)
	}

	withDrafts := preparePosts(posts, cfg, true)
	if len(withDrafts) != 2 {
		t.Fatalf("Expected 2 visible posts with drafts enabled, got %d", len(withDrafts))
	}
}

func TestPrepareRenderableContent_FiltersAndSortsPosts(t *testing.T) {
	posts := []*parser.Post{
		{Title: "draft", Slug: "draft", Date: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), Draft: true},
		{Title: "older", Slug: "older", Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{Title: "newer", Slug: "newer", Date: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)},
	}

	state := prepareRenderableContent(posts, nil, &config.Config{}, false)
	if len(state.posts) != 2 {
		t.Fatalf("Expected 2 visible posts, got %d", len(state.posts))
	}
	if state.posts[0].Slug != "newer" || state.posts[1].Slug != "older" {
		t.Fatalf("Expected visible posts to be sorted desc, got %#v", []string{state.posts[0].Slug, state.posts[1].Slug})
	}
}

func TestAssembleGenerationPlan_UsesProvidedPlans(t *testing.T) {
	tmpl := template.Must(template.New("singlePage").Parse(`{{ define "singlePage" }}ok{{ end }}`))
	pageSpecs := []PageSpec{{
		TemplateCandidates: []string{"singlePage"},
		OutputPath:         "post/index.html",
		Data:               "ok",
	}}
	artifacts := []ArtifactSpec{{
		Name:    "search",
		Enabled: true,
		Run:     func() error { return nil },
	}}

	plan := assembleGenerationPlan("public", tmpl, pageBuildResult{pageSpecs: pageSpecs}, artifacts, true)
	if plan.outputDir != "public" {
		t.Fatalf("Expected outputDir public, got %q", plan.outputDir)
	}
	if len(plan.pagePlan.pages) != 1 {
		t.Fatalf("Expected 1 page spec, got %d", len(plan.pagePlan.pages))
	}
	if len(plan.artifacts.specs) != 1 || plan.artifacts.specs[0].Name != "search" {
		t.Fatalf("Expected artifact plan to keep provided artifacts, got %#v", plan.artifacts.specs)
	}
}

func TestDetectStylesheetPath(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if err := os.MkdirAll(filepath.Join(tmpDir, "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create assets directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "assets", "css", "style.css"), []byte("body{}"), 0644); err != nil {
		t.Fatalf("Failed to create stylesheet: %v", err)
	}

	got := detectStylesheetPath(&config.Config{StaticDir: "assets"})
	want := "/css/style.css"
	if got != want {
		t.Fatalf("Expected stylesheet path %q, got %q", want, got)
	}
}

func TestDetectStylesheetPath_SiteAssetsOverrideTheme(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if err := os.MkdirAll(filepath.Join(tmpDir, "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create site assets directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create theme assets directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "assets", "css", "style.css"), []byte("body{}"), 0644); err != nil {
		t.Fatalf("Failed to create site stylesheet: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "themes", "demo", "assets", "css", "main.css"), []byte("body{}"), 0644); err != nil {
		t.Fatalf("Failed to create theme stylesheet: %v", err)
	}

	got := detectStylesheetPath(&config.Config{
		Theme:     "demo",
		ThemesDir: "themes",
		StaticDir: "assets",
	})
	want := "/css/style.css"
	if got != want {
		t.Fatalf("Expected stylesheet path %q, got %q", want, got)
	}
}

func TestDetectStylesheetPath_EmptyWhenNoStylesheetExists(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if err := os.MkdirAll(filepath.Join(tmpDir, "assets", "images"), 0755); err != nil {
		t.Fatalf("Failed to create assets directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "assets", "images", "logo.svg"), []byte("<svg/>"), 0644); err != nil {
		t.Fatalf("Failed to create image asset: %v", err)
	}

	got := detectStylesheetPath(&config.Config{StaticDir: "assets"})
	if got != "" {
		t.Fatalf("Expected no stylesheet path, got %q", got)
	}
}

func TestCollectStaticAssetFiles_SiteOverridesTheme(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if err := os.MkdirAll(filepath.Join(tmpDir, "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create site asset directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create theme asset directory: %v", err)
	}
	mustWriteFile(t, filepath.Join(tmpDir, "assets", "css", "main.css"), "site")
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "assets", "css", "main.css"), "theme")
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "assets", "css", "syntax.css"), "syntax")

	assets, err := collectStaticAssetFiles(&config.Config{
		Theme:     "demo",
		ThemesDir: "themes",
		StaticDir: "assets",
	})
	if err != nil {
		t.Fatalf("collectStaticAssetFiles failed: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("Expected 2 assets after overlay, got %d", len(assets))
	}
	if assets[0].OutputPath != filepath.Join("css", "main.css") || assets[0].SourcePath != filepath.Join("assets", "css", "main.css") {
		t.Fatalf("Expected site asset to win for css/main.css, got %#v", assets[0])
	}
	if assets[1].OutputPath != filepath.Join("css", "syntax.css") {
		t.Fatalf("Expected syntax.css to remain from theme, got %#v", assets[1])
	}
}

func TestPlanStaticAssetCopies_SkipsCurrentDestination(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
	mustWriteFile(t, sourcePath, "body{}")
	outputDir := filepath.Join(tmpDir, "public")
	destPath := filepath.Join(outputDir, "css", "main.css")
	mustWriteFile(t, destPath, "body{}")

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	future := sourceInfo.ModTime().Add(time.Hour)
	if err := os.Chtimes(destPath, future, future); err != nil {
		t.Fatalf("set dest time: %v", err)
	}

	plans, err := planStaticAssetCopies(&config.Config{StaticDir: "assets"}, outputDir)
	if err != nil {
		t.Fatalf("planStaticAssetCopies failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("Expected 1 plan, got %d", len(plans))
	}
	if plans[0].Action != staticAssetSkip || plans[0].Reason != "current" {
		t.Fatalf("Expected current asset to be skipped, got %#v", plans[0])
	}
}

func TestPlanStaticAssetCopies_CopiesChangedAssets(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, sourcePath string, destPath string)
		want       staticAssetCopyAction
		wantReason string
	}{
		{
			name: "missing destination",
			setup: func(t *testing.T, sourcePath string, destPath string) {
				mustWriteFile(t, sourcePath, "body{}")
			},
			want:       staticAssetCopy,
			wantReason: "missing",
		},
		{
			name: "size differs",
			setup: func(t *testing.T, sourcePath string, destPath string) {
				mustWriteFile(t, sourcePath, "body{}")
				mustWriteFile(t, destPath, "different")
			},
			want:       staticAssetCopy,
			wantReason: "size",
		},
		{
			name: "mode differs",
			setup: func(t *testing.T, sourcePath string, destPath string) {
				mustWriteFile(t, sourcePath, "body{}")
				mustWriteFile(t, destPath, "body{}")
				if err := os.Chmod(sourcePath, 0600); err != nil {
					t.Fatalf("chmod source: %v", err)
				}
				if err := os.Chmod(destPath, 0644); err != nil {
					t.Fatalf("chmod dest: %v", err)
				}
			},
			want:       staticAssetCopy,
			wantReason: "mode",
		},
		{
			name: "source newer",
			setup: func(t *testing.T, sourcePath string, destPath string) {
				mustWriteFile(t, sourcePath, "body{}")
				mustWriteFile(t, destPath, "body{}")
				past := time.Now().Add(-time.Hour)
				future := time.Now().Add(time.Hour)
				if err := os.Chtimes(destPath, past, past); err != nil {
					t.Fatalf("set dest time: %v", err)
				}
				if err := os.Chtimes(sourcePath, future, future); err != nil {
					t.Fatalf("set source time: %v", err)
				}
			},
			want:       staticAssetCopy,
			wantReason: "source-newer",
		},
		{
			name: "content differs with same metadata",
			setup: func(t *testing.T, sourcePath string, destPath string) {
				mustWriteFile(t, sourcePath, "new!")
				mustWriteFile(t, destPath, "old!")
				sourceInfo, err := os.Stat(sourcePath)
				if err != nil {
					t.Fatalf("stat source: %v", err)
				}
				future := sourceInfo.ModTime().Add(time.Hour)
				if err := os.Chtimes(destPath, future, future); err != nil {
					t.Fatalf("set dest time: %v", err)
				}
			},
			want:       staticAssetCopy,
			wantReason: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Chdir(tmpDir)
			sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
			destPath := filepath.Join(tmpDir, "public", "css", "main.css")
			tt.setup(t, sourcePath, destPath)

			plans, err := planStaticAssetCopies(&config.Config{StaticDir: "assets"}, filepath.Join(tmpDir, "public"))
			if err != nil {
				t.Fatalf("planStaticAssetCopies failed: %v", err)
			}
			if len(plans) != 1 {
				t.Fatalf("Expected 1 plan, got %d", len(plans))
			}
			if plans[0].Action != tt.want || plans[0].Reason != tt.wantReason {
				t.Fatalf("Expected %s/%s, got %#v", tt.want, tt.wantReason, plans[0])
			}
		})
	}
}

func TestCopyStaticAssets_PreservesSourcePermissionsForSkip(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
	destPath := filepath.Join(outputDir, "css", "main.css")
	mustWriteFile(t, sourcePath, "body{}")
	if err := os.Chmod(sourcePath, 0600); err != nil {
		t.Fatalf("chmod source: %v", err)
	}

	t.Chdir(tmpDir)
	cfg := &config.Config{StaticDir: "assets"}
	if err := copyStaticAssets(cfg, outputDir); err != nil {
		t.Fatalf("first copyStaticAssets failed: %v", err)
	}

	destInfo, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if destInfo.Mode().Perm() != 0600 {
		t.Fatalf("Expected copied asset mode 0600, got %v", destInfo.Mode().Perm())
	}

	plans, err := planStaticAssetCopies(cfg, outputDir)
	if err != nil {
		t.Fatalf("planStaticAssetCopies failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("Expected 1 plan, got %d", len(plans))
	}
	if plans[0].Action != staticAssetSkip || plans[0].Reason != "current" {
		t.Fatalf("Expected preserved-mode asset to be skipped, got %#v", plans[0])
	}
}

func TestExecuteStaticAssetCopyPlan_SkipsCurrentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
	destPath := filepath.Join(tmpDir, "public", "css", "main.css")
	mustWriteFile(t, sourcePath, "body{}")
	mustWriteFile(t, destPath, "body{}")

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	destTime := sourceInfo.ModTime().Add(time.Hour)
	if err := os.Chtimes(destPath, destTime, destTime); err != nil {
		t.Fatalf("set dest time: %v", err)
	}
	before, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest before: %v", err)
	}

	plans, err := planStaticAssetCopies(&config.Config{StaticDir: "assets"}, filepath.Join(tmpDir, "public"))
	if err != nil {
		t.Fatalf("planStaticAssetCopies failed: %v", err)
	}
	result, err := executeStaticAssetCopyPlan(plans)
	if err != nil {
		t.Fatalf("executeStaticAssetCopyPlan failed: %v", err)
	}
	if result.Copied != 0 || result.Skipped != 1 {
		t.Fatalf("Expected copied=0 skipped=1, got %#v", result)
	}

	after, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest after: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("Expected skipped file modtime to remain %v, got %v", before.ModTime(), after.ModTime())
	}
}

func TestExecuteStaticAssetCopyPlan_CopiesChangedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
	destPath := filepath.Join(tmpDir, "public", "css", "main.css")
	mustWriteFile(t, sourcePath, "new")
	mustWriteFile(t, destPath, "old")
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(destPath, past, past); err != nil {
		t.Fatalf("set dest time: %v", err)
	}
	if err := os.Chtimes(sourcePath, future, future); err != nil {
		t.Fatalf("set source time: %v", err)
	}

	plans, err := planStaticAssetCopies(&config.Config{StaticDir: "assets"}, filepath.Join(tmpDir, "public"))
	if err != nil {
		t.Fatalf("planStaticAssetCopies failed: %v", err)
	}
	result, err := executeStaticAssetCopyPlan(plans)
	if err != nil {
		t.Fatalf("executeStaticAssetCopyPlan failed: %v", err)
	}
	if result.Copied != 1 || result.Skipped != 0 {
		t.Fatalf("Expected copied=1 skipped=0, got %#v", result)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("Expected copied content %q, got %q", "new", string(content))
	}
}

func TestBuildSearchDocuments(t *testing.T) {
	posts := []*parser.Post{
		{
			Title:      "Alpha",
			URL:        "/alpha/",
			Date:       time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
			Tags:       []string{"go"},
			Summary:    "alpha summary",
			Content:    "alpha    content\nwith spacing",
			Categories: []string{"tech"},
		},
	}
	cfg := &config.Config{Author: "mengbin"}

	full := buildSearchDocuments(posts, cfg, true)
	minimal := buildSearchDocuments(posts, cfg, false)

	if len(full) != 1 || len(minimal) != 1 {
		t.Fatalf("Expected one search document in each variant, got %d and %d", len(full), len(minimal))
	}
	if full[0].Title != minimal[0].Title || full[0].URL != minimal[0].URL || full[0].Category != "tech" {
		t.Fatalf("Expected shared metadata to match, got full=%#v minimal=%#v", full[0], minimal[0])
	}
	if full[0].Content != "alpha content with spacing" {
		t.Fatalf("Expected normalized content in full search doc, got %q", full[0].Content)
	}
	if minimal[0].Content != "" {
		t.Fatalf("Expected minimal search doc to omit content, got %q", minimal[0].Content)
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

func TestCopyStaticAssets_MissingSiteAssetsDir(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "official-website", "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create theme assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "themes", "official-website", "assets", "css", "theme.css"), []byte("body{}"), 0644); err != nil {
		t.Fatalf("Failed to create theme stylesheet: %v", err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Theme:     "official-website",
		ThemesDir: "themes",
		StaticDir: "assets",
	}

	if err := copyStaticAssets(cfg, outputDir); err != nil {
		t.Fatalf("Expected missing site assets dir to be ignored, got error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "css", "theme.css")); err != nil {
		t.Fatalf("Expected theme asset to be copied, got error: %v", err)
	}
}

func TestCopyStaticAssets_SiteAssetsOverrideThemeAssets(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create theme assets dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "assets", "css"), 0755); err != nil {
		t.Fatalf("Failed to create site assets dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "assets", "css", "theme.css"), "theme-version")
	mustWriteFile(t, filepath.Join(tmpDir, "assets", "css", "theme.css"), "site-version")

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Theme:     "demo",
		ThemesDir: "themes",
		StaticDir: "assets",
	}

	if err := copyStaticAssets(cfg, outputDir); err != nil {
		t.Fatalf("copyStaticAssets failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outputDir, "css", "theme.css"))
	if err != nil {
		t.Fatalf("Failed to read copied asset: %v", err)
	}
	if string(content) != "site-version" {
		t.Fatalf("Expected site asset to override theme asset, got %q", string(content))
	}
}

func TestCopyStaticAssets_SkipsCurrentAssets(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	sourcePath := filepath.Join(tmpDir, "assets", "css", "main.css")
	destPath := filepath.Join(outputDir, "css", "main.css")
	mustWriteFile(t, sourcePath, "body{}")

	t.Chdir(tmpDir)
	cfg := &config.Config{StaticDir: "assets"}
	if err := copyStaticAssets(cfg, outputDir); err != nil {
		t.Fatalf("first copyStaticAssets failed: %v", err)
	}

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	destTime := sourceInfo.ModTime().Add(time.Hour)
	if err := os.Chtimes(destPath, destTime, destTime); err != nil {
		t.Fatalf("set dest time: %v", err)
	}
	before, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest before: %v", err)
	}

	if err := copyStaticAssets(cfg, outputDir); err != nil {
		t.Fatalf("second copyStaticAssets failed: %v", err)
	}
	after, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest after: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("Expected current asset not to be rewritten; before=%v after=%v", before.ModTime(), after.ModTime())
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

	baseTemplate := `{{ define "base" }}{{ render .HeaderTemplate . }}{{ render .MainTemplate . }}{{ render .FooterTemplate . }}{{ end }}`

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

	template404 := `{{ define "notFoundMain" }}404{{ end }}
{{ define "notFoundPage" }}
<!DOCTYPE html>
<html>
<head><title>404</title></head>
<body>Not Found</body>
</html>
{{ end }}`

	os.WriteFile(filepath.Join(defaultDir, "base.html"), []byte(baseTemplate), 0644)
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
		Theme:      "",
		StaticDir:  "assets",
		PublishDir: "public",
		ThemesDir:  "themes",
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
		Theme:      "",
		StaticDir:  "assets",
		PublishDir: "public",
		ThemesDir:  "themes",
	}

	_, err := loadTemplates(cfg)
	if err == nil {
		t.Error("Expected error when no templates are found")
	}
}

func TestLoadTemplates_WithCommentAndAnalyticsPartials(t *testing.T) {
	tmpDir := t.TempDir()

	templatesDir := filepath.Join(tmpDir, "templates")
	defaultDir := filepath.Join(templatesDir, "_default")
	partialsDir := filepath.Join(templatesDir, "partials")

	os.MkdirAll(defaultDir, 0755)
	os.MkdirAll(partialsDir, 0755)

	os.WriteFile(filepath.Join(defaultDir, "base.html"), []byte(`{{ define "base" }}ok{{ end }}`), 0644)
	os.WriteFile(filepath.Join(defaultDir, "single.html"), []byte(`{{ define "singlePage" }}{{ template "comments" . }}{{ end }}`), 0644)
	os.WriteFile(filepath.Join(defaultDir, "list.html"), []byte(`{{ define "listPage" }}ok{{ end }}`), 0644)
	os.WriteFile(filepath.Join(defaultDir, "404.html"), []byte(`{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}404{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "header.html"), []byte(`{{ define "header" }}h{{ end }}{{ define "headerNested" }}h{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "footer.html"), []byte(`{{ define "footer" }}f{{ end }}{{ define "footerNested" }}f{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "comments.html"), []byte(`{{ define "comments" }}{{ "x" | default "fallback" }}{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "analytics.html"), []byte(`{{ define "analytics" }}a{{ end }}`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{}
	if _, err := loadTemplates(cfg); err != nil {
		t.Fatalf("Expected templates with comment and analytics partials to load, got error: %v", err)
	}
}

func TestGetTemplatePaths_ThemeTemplatesOverrideSite(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}site{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "single.html"), `{{ define "singlePage" }}theme{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}theme-base{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "partials", "header.html"), `{{ define "header" }}theme-header{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	paths := getTemplatePaths(&config.Config{Theme: "demo", ThemesDir: "themes"})
	expected := []string{
		filepath.Join("templates", "_default", "single.html"),
		filepath.Join("themes", "demo", "layouts", "_default", "single.html"),
		filepath.Join("themes", "demo", "layouts", "_default", "base.html"),
		filepath.Join("themes", "demo", "layouts", "partials", "header.html"),
	}

	for _, want := range expected {
		if !slices.Contains(paths, want) {
			t.Fatalf("Expected template paths to contain %s, got %#v", want, paths)
		}
	}
	siteSingle := slices.Index(paths, filepath.Join("templates", "_default", "single.html"))
	themeSingle := slices.Index(paths, filepath.Join("themes", "demo", "layouts", "_default", "single.html"))
	if siteSingle == -1 || themeSingle == -1 || themeSingle <= siteSingle {
		t.Fatalf("Expected theme template to be loaded after site template for override, got %#v", paths)
	}
}

func TestLoadTemplates_ThemeTemplatesOverrideSite(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}site{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listPage" }}list{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}site-header{{ end }}{{ define "headerNested" }}site-header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}site-footer{{ end }}{{ define "footerNested" }}site-footer{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "single.html"), `{{ define "singleMain" }}theme{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "singlePage", SinglePageData{
		BasePageData: BasePageData{MainTemplate: "singleMain"},
	}); err != nil {
		t.Fatalf("Failed to render singlePage: %v", err)
	}

	if buf.String() != "theme" {
		t.Fatalf("Expected theme template to override site template, got %q", buf.String())
	}
}

func TestLoadTemplates_ThemePartialsOverrideSite(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .HeaderTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listPage" }}list{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}site-header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "partials", "header.html"), `{{ define "header" }}theme-header{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "singlePage", SinglePageData{
		BasePageData: BasePageData{HeaderTemplate: "header"},
	}); err != nil {
		t.Fatalf("Failed to render singlePage: %v", err)
	}

	if buf.String() != "theme-header" {
		t.Fatalf("Expected theme partial to override site partial, got %q", buf.String())
	}
}

func TestLoadTemplates_ThemePartialsFallbackWhenSiteMissing(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .HeaderTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listPage" }}list{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "partials", "header.html"), `{{ define "header" }}theme-header{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "singlePage", SinglePageData{
		BasePageData: BasePageData{HeaderTemplate: "header"},
	}); err != nil {
		t.Fatalf("Failed to render singlePage: %v", err)
	}

	if buf.String() != "theme-header" {
		t.Fatalf("Expected theme partial fallback when site partial missing, got %q", buf.String())
	}
}

func TestLoadTemplates_ThemeMissingSingleFallsBackToSiteSingle(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}site-single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listMain" }}site-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}site-header{{ end }}{{ define "headerNested" }}site-header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}site-footer{{ end }}{{ define "footerNested" }}site-footer{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "list.html"), `{{ define "listMain" }}theme-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "singlePage", SinglePageData{
		BasePageData: BasePageData{MainTemplate: "singleMain"},
	}); err != nil {
		t.Fatalf("Failed to render singlePage: %v", err)
	}

	if buf.String() != "site-single" {
		t.Fatalf("Expected site single template fallback when theme single missing, got %q", buf.String())
	}
}

func TestLoadTemplates_ThemeMissingTaxonomyFallsBackToSiteTaxonomy(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}site-single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listMain" }}site-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}site-404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}site-terms{{ end }}{{ define "taxonomyMain" }}site-taxonomy{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}site-header{{ end }}{{ define "headerNested" }}site-header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}site-footer{{ end }}{{ define "footerNested" }}site-footer{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "single.html"), `{{ define "singleMain" }}theme-single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "list.html"), `{{ define "listMain" }}theme-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "taxonomyPage", TaxonomyPageData{
		BasePageData: BasePageData{MainTemplate: "taxonomyMain"},
	}); err != nil {
		t.Fatalf("Failed to render taxonomyPage: %v", err)
	}

	if buf.String() != "site-taxonomy" {
		t.Fatalf("Expected site taxonomy template fallback when theme taxonomy missing, got %q", buf.String())
	}
}

func TestLoadTemplates_ThemeMissingNotFoundFallsBackToSiteNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}site-single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listMain" }}site-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}site-notfound{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}site-terms{{ end }}{{ define "taxonomyMain" }}site-taxonomy{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}site-header{{ end }}{{ define "headerNested" }}site-header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}site-footer{{ end }}{{ define "footerNested" }}site-footer{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "single.html"), `{{ define "singleMain" }}theme-single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "layouts", "_default", "list.html"), `{{ define "listMain" }}theme-list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tpl, err := loadTemplates(&config.Config{Theme: "demo", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "notFoundPage", NotFoundPageData{
		BasePageData: BasePageData{MainTemplate: "notFoundMain"},
	}); err != nil {
		t.Fatalf("Failed to render notFoundPage: %v", err)
	}

	if buf.String() != "site-notfound" {
		t.Fatalf("Expected site notFound template fallback when theme 404 missing, got %q", buf.String())
	}
}

func TestGenerate_MinimalTemplateSet(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "public")

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .HeaderTemplate . }}|{{ render .MainTemplate . }}|{{ render .FooterTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listMain" }}list:{{ len .Posts }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}single:{{ .Post.Title }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}notfound{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}terms:{{ .Kind }}:{{ len .Terms }}{{ end }}{{ define "taxonomyMain" }}taxonomy:{{ .Term }}:{{ len .Posts }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}header{{ end }}{{ define "headerNested" }}header-nested{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}footer{{ end }}{{ define "footerNested" }}footer-nested{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Minimal Post",
			Slug:        "minimal-post",
			Date:        time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Minimal content.</p>",
			Tags:        []string{"Go"},
			Categories:  []string{"Tech"},
		},
	}

	cfg := &config.Config{
		Title:        "Minimal Site",
		Description:  "Minimal template coverage.",
		BaseURL:      "https://example.com",
		StaticDir:    "assets",
		Paginate:     10,
		PaginatePath: "page",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed with minimal templates: %v", err)
	}

	checks := map[string]string{
		"index.html": "header|list:1|footer",
		filepath.Join("minimal-post", "index.html"): "header-nested|single:Minimal Post|footer-nested",
		"404.html":                          "header|notfound|footer",
		filepath.Join("tags", "index.html"): "header|terms:tags:1|footer",
	}

	for relPath, expected := range checks {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		if string(content) != expected {
			t.Fatalf("Expected %s to equal %q, got %q", relPath, expected, string(content))
		}
	}
}

func TestGenerate_ThemeMissingTaxonomyAndNotFoundFallsBackToSiteTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := os.MkdirAll(filepath.Join(siteDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create site templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create site partials dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "themes", "demo", "layouts", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create theme templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "themes", "demo", "layouts", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create theme partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}site-notfound{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}site-terms:{{ .Kind }}:{{ len .Terms }}{{ end }}{{ define "taxonomyMain" }}site-taxonomy:{{ .Term }}:{{ len .Posts }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "comments.html"), `{{ define "comments" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "analytics.html"), `{{ define "analytics" }}{{ end }}`)

	mustWriteFile(t, filepath.Join(siteDir, "themes", "demo", "layouts", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "themes", "demo", "layouts", "_default", "single.html"), `{{ define "singleMain" }}theme-single:{{ .Post.Title }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "themes", "demo", "layouts", "_default", "list.html"), `{{ define "listMain" }}theme-list:{{ len .Posts }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "themes", "demo", "layouts", "partials", "header.html"), `{{ define "header" }}theme-header{{ end }}{{ define "headerNested" }}theme-header{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "themes", "demo", "layouts", "partials", "footer.html"), `{{ define "footer" }}theme-footer{{ end }}{{ define "footerNested" }}theme-footer{{ end }}`)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Fallback Post",
			Slug:        "fallback-post",
			Date:        time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Fallback content.</p>",
			Tags:        []string{"Fallback Tag"},
		},
	}

	cfg := &config.Config{
		Title:        "Fallback Site",
		Description:  "Theme fallback integration coverage.",
		BaseURL:      "https://example.com",
		Theme:        "demo",
		ThemesDir:    "themes",
		StaticDir:    "assets",
		Paginate:     10,
		PaginatePath: "page",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	taxonomyContent, err := os.ReadFile(filepath.Join(outputDir, "tags", "fallback-tag", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read taxonomy output: %v", err)
	}
	if string(taxonomyContent) != "site-taxonomy:fallback tag:1" {
		t.Fatalf("Expected taxonomy page to use site fallback template, got %q", string(taxonomyContent))
	}

	notFoundContent, err := os.ReadFile(filepath.Join(outputDir, "404.html"))
	if err != nil {
		t.Fatalf("Failed to read 404 output: %v", err)
	}
	if string(notFoundContent) != "site-notfound" {
		t.Fatalf("Expected 404 page to use site fallback template, got %q", string(notFoundContent))
	}
}

func TestAnalyticsPartialRendersConfiguredProvider(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to locate repository root: %v", err)
	}

	analyticsBytes, err := os.ReadFile(filepath.Join(repoRoot, "templates", "partials", "analytics.html"))
	if err != nil {
		t.Fatalf("Failed to read analytics partial: %v", err)
	}

	tpl, err := template.New("analytics").Funcs(template.FuncMap{
		"default": func(fallback, value interface{}) interface{} {
			switch v := value.(type) {
			case string:
				if v == "" {
					return fallback
				}
			case nil:
				return fallback
			}
			return value
		},
	}).Parse(string(analyticsBytes))
	if err != nil {
		t.Fatalf("Failed to parse analytics partial: %v", err)
	}

	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "google analytics",
			cfg: &config.Config{
				Analytics: &config.AnalyticsConfig{
					Provider: "google",
					Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-TEST123"},
				},
			},
			want: "googletagmanager.com/gtag/js?id=G-TEST123",
		},
		{
			name: "baidu analytics",
			cfg: &config.Config{
				Analytics: &config.AnalyticsConfig{
					Provider: "baidu",
					Baidu:    &config.BaiduAnalyticsConfig{TrackingID: "baidu-123"},
				},
			},
			want: "hm.baidu.com/hm.js?baidu-123",
		},
		{
			name: "matomo analytics",
			cfg: &config.Config{
				Analytics: &config.AnalyticsConfig{
					Provider: "matomo",
					Matomo: &config.MatomoAnalyticsConfig{
						URL:    "https://analytics.example.com",
						SiteID: 7,
					},
				},
			},
			want: "setSiteId', '7'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tpl.ExecuteTemplate(&buf, "analytics", BasePageData{Site: tt.cfg}); err != nil {
				t.Fatalf("Failed to execute analytics partial: %v", err)
			}

			if !strings.Contains(buf.String(), tt.want) {
				t.Fatalf("Expected rendered analytics partial to contain %q, got %s", tt.want, buf.String())
			}
		})
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

func TestRenderHelperReturnsTemplateExecutionError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}before {{ render "missingPartial" . }} after{{ end }}`)

	tpl, err := loadTemplates(&config.Config{})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	err = renderTemplate(tpl, "singlePage", filepath.Join(tmpDir, "out.html"), map[string]string{})
	if err == nil {
		t.Fatal("Expected renderTemplate to fail when render helper targets a missing template")
	}
	if !strings.Contains(err.Error(), "missingPartial") {
		t.Fatalf("Expected error to mention missing partial, got %v", err)
	}
}

func TestRenderPageSpecs_UsesFirstAvailableTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	tpl, err := template.New("pages").Parse(`{{ define "fallback" }}fallback: {{ . }}{{ end }}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	if err := renderPageSpecs(tpl, tmpDir, []PageSpec{{
		TemplateCandidates: []string{"missing", "fallback"},
		OutputPath:         "nested/index.html",
		Data:               "ok",
	}}); err != nil {
		t.Fatalf("renderPageSpecs failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "nested", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read rendered page: %v", err)
	}

	if string(content) != "fallback: ok" {
		t.Fatalf("Unexpected rendered content: %s", string(content))
	}
}

func TestBuildPageSpecs(t *testing.T) {
	post := &parser.Post{
		Title:       "Spec Post",
		Description: "Spec Description",
		URL:         "/spec-post/",
		Date:        time.Now(),
		Tags:        []string{"Go"},
		Categories:  []string{"Tech"},
	}

	pages, tags, categories := buildPageSpecs([]*parser.Post{post}, &config.Config{
		Title:        "Spec Blog",
		Description:  "Spec Site",
		BaseURL:      "https://example.com",
		Paginate:     10,
		PaginatePath: "page",
	})

	if len(pages) != 7 {
		t.Fatalf("Expected 7 page specs, got %d", len(pages))
	}
	if len(tags) != 1 || tags[0] != "go" {
		t.Fatalf("Unexpected tags: %#v", tags)
	}
	if len(categories) != 1 || categories[0] != "tech" {
		t.Fatalf("Unexpected categories: %#v", categories)
	}
}

func TestBuildArtifactSpecs(t *testing.T) {
	specs := buildArtifactSpecs(nil, &config.Config{
		BaseURL:         "https://example.com",
		EnableRobotsTXT: true,
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(true),
			Search:  boolPtr(false),
			Sitemap: boolPtr(true),
			Robots:  boolPtr(true),
		},
	}, t.TempDir(), []string{"go"}, []string{"tech"})

	if len(specs) != 7 {
		t.Fatalf("Expected 7 artifact specs, got %d", len(specs))
	}

	expected := map[string]bool{
		"feed":    true,
		"sitemap": true,
		"search":  false,
		"robots":  true,
		"aliases": true,
		"assets":  true,
		"minify":  false,
	}

	for _, spec := range specs {
		want, ok := expected[spec.Name]
		if !ok {
			t.Fatalf("Unexpected artifact name %q", spec.Name)
		}
		if spec.Enabled != want {
			t.Fatalf("Expected artifact %q enabled=%t, got %t", spec.Name, want, spec.Enabled)
		}
	}
}

func TestBuildArtifactSpecs_RelativeBaseURLDisablesAbsoluteURLArtifacts(t *testing.T) {
	specs := buildArtifactSpecs(nil, &config.Config{
		BaseURL:         "/",
		EnableRobotsTXT: true,
	}, t.TempDir(), nil, nil)

	states := map[string]bool{}
	for _, spec := range specs {
		states[spec.Name] = spec.Enabled
	}

	if states["feed"] {
		t.Fatal("Expected feed artifact to be disabled for relative baseURL")
	}
	if states["sitemap"] {
		t.Fatal("Expected sitemap artifact to be disabled for relative baseURL")
	}
	if !states["search"] {
		t.Fatal("Expected search artifact to remain enabled for relative baseURL")
	}
	if !states["robots"] {
		t.Fatal("Expected robots artifact to remain enabled when enabled in config")
	}
	if !states["aliases"] {
		t.Fatal("Expected aliases artifact to remain enabled")
	}
	if !states["assets"] {
		t.Fatal("Expected assets artifact to remain enabled")
	}
	if states["minify"] {
		t.Fatal("Expected minify artifact to remain disabled by default")
	}
}

func TestWithMinifyArtifactEnabled(t *testing.T) {
	specs := buildArtifactSpecs(nil, &config.Config{}, t.TempDir(), nil, nil)
	updated := withMinifyArtifactEnabled(specs, true)

	minifyEnabled := false
	for _, spec := range updated {
		if spec.Name == "minify" {
			minifyEnabled = spec.Enabled
			break
		}
	}

	if !minifyEnabled {
		t.Fatal("Expected minify artifact to be enabled")
	}
}

func TestPrepareGenerationPlan(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "_default"), 0755); err != nil {
		t.Fatalf("Failed to create default templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create partials dir: %v", err)
	}

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listMain" }}list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "page.html"), `{{ define "pageMain" }}page{{ end }}{{ define "pagePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}terms{{ end }}{{ define "taxonomyMain" }}taxonomy{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "header.html"), `{{ define "header" }}header{{ end }}{{ define "headerNested" }}header{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "footer.html"), `{{ define "footer" }}footer{{ end }}{{ define "footerNested" }}footer{{ end }}`)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{Title: "Older", Slug: "older", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Tags: []string{"Go"}, Categories: []string{"Tech"}},
		{Title: "Newer", Slug: "newer", Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Tags: []string{"Go"}, Categories: []string{"Tech"}},
	}
	pages := []*parser.Page{{Title: "About", URL: "/about/"}}
	cfg := &config.Config{
		Title:           "Plan Site",
		Description:     "Plan Desc",
		BaseURL:         "https://example.com",
		Paginate:        10,
		PaginatePath:    "page",
		EnableRobotsTXT: true,
	}

	plan, err := prepareGenerationPlan(posts, pages, cfg, "public", true, false)
	if err != nil {
		t.Fatalf("prepareGenerationPlan failed: %v", err)
	}

	if plan.outputDir != "public" {
		t.Fatalf("Expected output dir public, got %q", plan.outputDir)
	}
	if plan.pagePlan.templates == nil {
		t.Fatal("Expected templates to be loaded")
	}
	if len(plan.pagePlan.pages) != 9 {
		t.Fatalf("Expected 9 page specs including standalone page, got %d", len(plan.pagePlan.pages))
	}

	minifyEnabled := false
	for _, spec := range plan.artifacts.specs {
		if spec.Name == "minify" {
			minifyEnabled = spec.Enabled
			break
		}
	}
	if !minifyEnabled {
		t.Fatal("Expected minify artifact to be enabled in generation plan")
	}
}

func TestArtifactPipelineExecute_WrapsNamedErrors(t *testing.T) {
	err := artifactPipeline{
		specs: []ArtifactSpec{
			{Name: "feed", Enabled: true, Run: func() error { return fmt.Errorf("boom") }},
		},
	}.Execute()
	if err == nil || !strings.Contains(err.Error(), "feed artifact: boom") {
		t.Fatalf("Expected wrapped artifact error, got %v", err)
	}
}

func TestArtifactPipelineExecute_SkipsDisabledSpecs(t *testing.T) {
	var ran bool
	err := artifactPipeline{
		specs: []ArtifactSpec{
			{Name: "search", Enabled: false, Run: func() error { ran = true; return nil }},
		},
	}.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if ran {
		t.Fatal("Expected disabled artifact not to run")
	}
}

func TestGenerationPlanExecute_Order(t *testing.T) {
	var steps []string
	plan := &generationPlan{
		outputDir: "public",
		pagePlan: pageRenderPlan{
			outputDir: "public",
			templates: mustParseTemplate(t, `{{ define "page" }}ok{{ end }}`),
			pages: []PageSpec{
				{
					TemplateCandidates: []string{"page"},
					OutputPath:         "index.html",
					Data:               nil,
				},
			},
		},
		artifacts: artifactPipeline{
			specs: []ArtifactSpec{
				{
					Name:    "assets",
					Enabled: true,
					Run: func() error {
						steps = append(steps, "artifacts")
						return nil
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	plan.outputDir = tmpDir
	plan.pagePlan.outputDir = tmpDir

	err := plan.executeWith(true, func(outputDir string, cleanOutput bool) error {
		steps = append(steps, "prepare")
		return os.MkdirAll(outputDir, 0755)
	}, func() error {
		steps = append(steps, "render")
		return plan.pagePlan.Execute()
	}, func() error {
		return plan.artifacts.Execute()
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(steps) != 3 || steps[0] != "prepare" || steps[1] != "render" || steps[2] != "artifacts" {
		t.Fatalf("Unexpected execution order: %#v", steps)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "index.html")); err != nil {
		t.Fatalf("Expected page render output, got %v", err)
	}
}

func mustParseTemplate(t *testing.T, src string) *template.Template {
	t.Helper()
	tpl, err := template.New("test").Parse(src)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tpl
}

func TestGenerate_DefaultSiteGolden(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to locate repository root: %v", err)
	}

	if err := createGoldenTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create golden test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Alpha Post",
			Slug:        "alpha-post",
			Date:        time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			Description: "Alpha description.",
			Summary:     "Alpha summary.",
			SummaryHTML: "<p>Alpha summary.</p>",
			Content:     "Alpha body content.",
			ContentHTML: "<p>Alpha body content.</p>",
			Tags:        []string{"Go"},
			Categories:  []string{"Tech"},
			ReadingTime: 1,
		},
		{
			Title:       "Beta Post",
			Slug:        "beta-post",
			Date:        time.Date(2026, 3, 18, 9, 30, 0, 0, time.UTC),
			Description: "Beta description.",
			Summary:     "Beta summary.",
			SummaryHTML: "<p>Beta summary.</p>",
			Content:     "Beta body content.",
			ContentHTML: "<p>Beta body content.</p>",
			Tags:        []string{"Go"},
			Categories:  []string{"Tech"},
			Aliases:     []string{"/old-beta/"},
			ReadingTime: 1,
		},
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
		Social: map[string]string{
			"github": "https://github.com/example",
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	expectedFiles := []string{
		".gobin-assets.json",
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

	assertGoldenSiteOutput(t, repoRoot, outputDir, "default_site", expectedFiles)
}

func createGoldenTestSite(siteDir string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	if err := copyDirForTest(filepath.Join(repoRoot, "templates", "_default"), filepath.Join(siteDir, "templates", "_default")); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "templates", "partials"), 0755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "header.html"), filepath.Join(siteDir, "templates", "partials", "header.html")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "footer.html"), filepath.Join(siteDir, "templates", "partials", "footer.html")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(siteDir, "templates", "partials", "comments.html"), []byte(`{{ define "comments" }}{{ end }}`), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(siteDir, "templates", "partials", "analytics.html"), []byte(`{{ define "analytics" }}{{ end }}`), 0644); err != nil {
		return err
	}

	return copyDirForTest(filepath.Join(repoRoot, "assets"), filepath.Join(siteDir, "assets"))
}

func createOfficialThemeTestSite(siteDir string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	if err := copyDirForTest(
		filepath.Join(repoRoot, "themes", "official-website"),
		filepath.Join(siteDir, "themes", "official-website"),
	); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(siteDir, "templates", "partials"), 0755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "comments.html"), filepath.Join(siteDir, "templates", "partials", "comments.html")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "analytics.html"), filepath.Join(siteDir, "templates", "partials", "analytics.html")); err != nil {
		return err
	}

	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "templates")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "assets")); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("failed to locate repository root")
		}
		dir = parent
	}
}

func copyDirForTest(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirForTest(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func listRelativeFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk output dir: %v", err)
	}
	return files
}

func normalizeGoldenContent(relPath, content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	normalizeEOF := func(s string) string {
		return strings.TrimRight(s, "\n") + "\n"
	}

	switch relPath {
	case "index.atom":
		re := regexp.MustCompile(`(?m)^  <updated>.*</updated>$`)
		return normalizeEOF(re.ReplaceAllString(content, "  <updated><BUILD_RFC3339></updated>"))
	case "index.xml":
		rePubDate := regexp.MustCompile(`(?m)^    <pubDate>.*</pubDate>$`)
		content = rePubDate.ReplaceAllString(content, "    <pubDate><BUILD_RFC1123Z></pubDate>")
		reBuildDate := regexp.MustCompile(`(?m)^    <lastBuildDate>.*</lastBuildDate>$`)
		return normalizeEOF(reBuildDate.ReplaceAllString(content, "    <lastBuildDate><BUILD_RFC1123Z></lastBuildDate>"))
	case "sitemap.xml":
		buildDate := time.Now().Format("2006-01-02")
		return normalizeEOF(strings.ReplaceAll(content, "<lastmod>"+buildDate+"</lastmod>", "<lastmod><BUILD_DATE></lastmod>"))
	default:
		return normalizeEOF(content)
	}
}

func assertGoldenSiteOutput(t *testing.T, repoRoot, outputDir, goldenName string, expectedFiles []string) {
	t.Helper()

	actualFiles := listRelativeFiles(t, outputDir)
	if strings.Join(actualFiles, "\n") != strings.Join(expectedFiles, "\n") {
		t.Fatalf("Generated file list mismatch.\nExpected:\n%s\n\nActual:\n%s", strings.Join(expectedFiles, "\n"), strings.Join(actualFiles, "\n"))
	}

	goldenDir := filepath.Join(repoRoot, "internal", "generator", "testdata", "golden", goldenName)
	for _, relPath := range expectedFiles {
		gotBytes, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read generated file %s: %v", relPath, err)
		}

		wantBytes, err := os.ReadFile(filepath.Join(goldenDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read golden file %s: %v", relPath, err)
		}

		got := normalizeGoldenContent(relPath, string(gotBytes))
		want := normalizeGoldenContent(relPath, string(wantBytes))
		if got != want {
			t.Fatalf("Golden mismatch for %s\nExpected:\n%s\n\nActual:\n%s", relPath, want, got)
		}
	}
}

func TestGenerate_OfficialWebsiteTheme(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to locate repository root: %v", err)
	}

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Theme Alpha",
			Slug:        "theme-alpha",
			Date:        time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			Description: "Theme alpha description.",
			Summary:     "Theme alpha summary.",
			SummaryHTML: "<p>Theme alpha summary.</p>",
			ContentHTML: "<p>Theme alpha content.</p>",
			Tags:        []string{"Go Lang"},
			Categories:  []string{"Release Notes"},
		},
		{
			Title:       "Theme Beta",
			Slug:        "theme-beta",
			Date:        time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
			Description: "Theme beta description.",
			Summary:     "Theme beta summary.",
			SummaryHTML: "<p>Theme beta summary.</p>",
			ContentHTML: "<p>Theme beta content.</p>",
			Tags:        []string{"Go Lang"},
			Categories:  []string{"Release Notes"},
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme golden coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      1,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	expectedFiles := []string{
		".gobin-assets.json",
		"404.html",
		"categories/index.html",
		"categories/release-notes/index.html",
		"css/syntax.css",
		"css/theme.css",
		"images/favicon.svg",
		"index.html",
		"js/theme.js",
		"page/2/index.html",
		"tags/go-lang/index.html",
		"tags/index.html",
		"theme-alpha/index.html",
		"theme-beta/index.html",
	}

	assertGoldenSiteOutput(t, repoRoot, outputDir, "official_theme", expectedFiles)
}

func TestGenerate_OfficialWebsiteTheme_CustomPermalinks(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Deep Theme Post",
			Slug:        "deep-theme-post",
			Date:        time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC),
			Description: "Deep theme description.",
			Summary:     "Deep theme summary.",
			SummaryHTML: "<p>Deep theme summary.</p>",
			ContentHTML: "<p>Deep theme content.</p>",
			Tags:        []string{"Go Deep"},
			Categories:  []string{"Deep Category"},
		},
	}

	cfg := &config.Config{
		Title:        "Gobin Official",
		Description:  "Official theme permalink test.",
		BaseURL:      "https://example.com",
		Theme:        "official-website",
		ThemesDir:    "themes",
		StaticDir:    "assets",
		Paginate:     10,
		PaginatePath: "page",
		Permalinks: map[string]string{
			"posts": "/:year/:month/:day/:slug/",
		},
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	deepPostPath := filepath.Join(outputDir, "2026", "04", "03", "deep-theme-post", "index.html")
	if _, err := os.Stat(deepPostPath); err != nil {
		t.Fatalf("Expected deep permalink output to exist, got error: %v", err)
	}

	tagPage, err := os.ReadFile(filepath.Join(outputDir, "tags", "go-deep", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read generated tag page: %v", err)
	}
	if !strings.Contains(string(tagPage), `href="/2026/04/03/deep-theme-post/"`) {
		t.Fatalf("Expected tag page to link to deep permalink, got:\n%s", string(tagPage))
	}

	categoryPage, err := os.ReadFile(filepath.Join(outputDir, "categories", "deep-category", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read generated category page: %v", err)
	}
	if !strings.Contains(string(categoryPage), `href="/2026/04/03/deep-theme-post/"`) {
		t.Fatalf("Expected category page to link to deep permalink, got:\n%s", string(categoryPage))
	}
}

func TestGenerate_OfficialWebsiteTheme_RelativeBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Relative Theme",
			Slug:        "relative-theme",
			Date:        time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			Summary:     "Relative summary.",
			Content:     "Relative content.",
			ContentHTML: "<p>Relative content.</p>",
			Tags:        []string{"Go"},
			Categories:  []string{"Tech"},
		},
	}

	cfg := &config.Config{
		Title:           "Gobin Official",
		Description:     "Official theme relative baseURL test.",
		BaseURL:         "/",
		Theme:           "official-website",
		ThemesDir:       "themes",
		StaticDir:       "assets",
		Paginate:        10,
		PaginatePath:    "page",
		EnableRobotsTXT: true,
		RepositoryURL:   "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Search: boolPtr(true),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "index.xml")); !os.IsNotExist(err) {
		t.Fatalf("Expected RSS feed to be skipped for relative baseURL, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "index.atom")); !os.IsNotExist(err) {
		t.Fatalf("Expected Atom feed to be skipped for relative baseURL, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "sitemap.xml")); !os.IsNotExist(err) {
		t.Fatalf("Expected sitemap to be skipped for relative baseURL, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "search-index.json")); err != nil {
		t.Fatalf("Expected search index to remain enabled, got error: %v", err)
	}
	robots, err := os.ReadFile(filepath.Join(outputDir, "robots.txt"))
	if err != nil {
		t.Fatalf("Failed to read robots.txt: %v", err)
	}
	if strings.Contains(string(robots), "Sitemap:") {
		t.Fatalf("Expected robots.txt to omit sitemap for relative baseURL, got:\n%s", string(robots))
	}
}

func TestGenerate_OfficialWebsiteTheme_SEOCommentsAndAnalytics(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to locate repository root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "templates", "partials"), 0755); err != nil {
		t.Fatalf("Failed to create partials dir: %v", err)
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "comments.html"), filepath.Join(siteDir, "templates", "partials", "comments.html")); err != nil {
		t.Fatalf("Failed to copy comments partial: %v", err)
	}
	if err := copyFile(filepath.Join(repoRoot, "templates", "partials", "analytics.html"), filepath.Join(siteDir, "templates", "partials", "analytics.html")); err != nil {
		t.Fatalf("Failed to copy analytics partial: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Theme Integrations",
			Slug:        "theme-integrations",
			Date:        time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
			Description: "Theme integrations description.",
			ContentHTML: "<p>Theme integrations content.</p>",
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme integration coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		SEO: &config.SEOConfig{
			Image: "https://example.com/images/og-test.png",
		},
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-THEME123"},
		},
		Comments: &config.CommentsConfig{
			Enabled:  true,
			Provider: "disqus",
			Disqus:   &config.DisqusConfig{Shortname: "gobin-test"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	indexContent := string(indexHTML)
	if !strings.Contains(indexContent, `property="og:image" content="https://example.com/images/og-test.png"`) {
		t.Fatalf("Expected theme homepage to contain og:image, got:\n%s", indexContent)
	}
	if !strings.Contains(indexContent, `name="twitter:image" content="https://example.com/images/og-test.png"`) {
		t.Fatalf("Expected theme homepage to contain twitter:image, got:\n%s", indexContent)
	}
	if !strings.Contains(indexContent, `googletagmanager.com/gtag/js?id=G-THEME123`) {
		t.Fatalf("Expected theme homepage to contain analytics snippet, got:\n%s", indexContent)
	}

	postHTML, err := os.ReadFile(filepath.Join(outputDir, "theme-integrations", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read post page: %v", err)
	}
	postContent := string(postHTML)
	if !strings.Contains(postContent, `id="disqus_thread"`) {
		t.Fatalf("Expected theme post page to contain comments container, got:\n%s", postContent)
	}
	if !strings.Contains(postContent, `gobin-test.disqus.com/embed.js`) {
		t.Fatalf("Expected theme post page to contain Disqus embed, got:\n%s", postContent)
	}
}

func TestGenerate_OfficialWebsiteTheme_CustomPermalinksWithCommentsAndAnalytics(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Deep Integrations",
			Slug:        "deep-integrations",
			Date:        time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
			Description: "Deep integration description.",
			ContentHTML: "<p>Deep integration content.</p>",
		},
	}

	cfg := &config.Config{
		Title:        "Gobin Official",
		Description:  "Official theme deep integration coverage.",
		BaseURL:      "https://example.com",
		Theme:        "official-website",
		ThemesDir:    "themes",
		StaticDir:    "assets",
		Paginate:     10,
		PaginatePath: "page",
		Permalinks: map[string]string{
			"posts": "/:year/:month/:day/:slug/",
		},
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-DEEP123"},
		},
		Comments: &config.CommentsConfig{
			Enabled:  true,
			Provider: "disqus",
			Disqus:   &config.DisqusConfig{Shortname: "gobin-deep"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	postHTML, err := os.ReadFile(filepath.Join(outputDir, "2026", "04", "05", "deep-integrations", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read deep permalink post page: %v", err)
	}

	postContent := string(postHTML)
	if !strings.Contains(postContent, `this.page.url = 'https:\/\/example.com\/2026\/04\/05\/deep-integrations\/'`) {
		t.Fatalf("Expected Disqus config to use deep permalink URL, got:\n%s", postContent)
	}
	if !strings.Contains(postContent, `gobin-deep.disqus.com/embed.js`) {
		t.Fatalf("Expected post page to contain Disqus embed, got:\n%s", postContent)
	}
	if !strings.Contains(postContent, `googletagmanager.com/gtag/js?id=G-DEEP123`) {
		t.Fatalf("Expected post page to contain analytics snippet, got:\n%s", postContent)
	}
}

func TestGenerate_OfficialWebsiteTheme_DraftsWithCommentsAndAnalytics(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Visible Integrated Post",
			Slug:        "visible-integrated-post",
			Date:        time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Visible integrated content.</p>",
		},
		{
			Title:       "Draft Integrated Post",
			Slug:        "draft-integrated-post",
			Date:        time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Draft integrated content.</p>",
			Draft:       true,
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme draft integration coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-DRAFT123"},
		},
		Comments: &config.CommentsConfig{
			Enabled:  true,
			Provider: "disqus",
			Disqus:   &config.DisqusConfig{Shortname: "gobin-draft"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate without drafts failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "draft-integrated-post", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Expected draft post to be absent by default, got err=%v", err)
	}

	visibleHTML, err := os.ReadFile(filepath.Join(outputDir, "visible-integrated-post", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read visible post page: %v", err)
	}
	visibleContent := string(visibleHTML)
	if !strings.Contains(visibleContent, `gobin-draft.disqus.com/embed.js`) {
		t.Fatalf("Expected visible post page to contain Disqus embed, got:\n%s", visibleContent)
	}
	if !strings.Contains(visibleContent, `googletagmanager.com/gtag/js?id=G-DRAFT123`) {
		t.Fatalf("Expected visible post page to contain analytics snippet, got:\n%s", visibleContent)
	}

	if err := Generate(posts, cfg, outputDir, false, true, true); err != nil {
		t.Fatalf("Generate with drafts failed: %v", err)
	}

	draftHTML, err := os.ReadFile(filepath.Join(outputDir, "draft-integrated-post", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read draft post page: %v", err)
	}
	draftContent := string(draftHTML)
	if !strings.Contains(draftContent, `gobin-draft.disqus.com/embed.js`) {
		t.Fatalf("Expected draft post page to contain Disqus embed when drafts enabled, got:\n%s", draftContent)
	}
	if !strings.Contains(draftContent, `googletagmanager.com/gtag/js?id=G-DRAFT123`) {
		t.Fatalf("Expected draft post page to contain analytics snippet when drafts enabled, got:\n%s", draftContent)
	}
}

func TestGenerate_OfficialWebsiteTheme_PaginationWithSEOAndAnalytics(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{Title: "Paged One", Slug: "paged-one", Date: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC), ContentHTML: "<p>One</p>"},
		{Title: "Paged Two", Slug: "paged-two", Date: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC), ContentHTML: "<p>Two</p>"},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme pagination integration coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      1,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		SEO: &config.SEOConfig{
			Image: "https://example.com/images/paged-og.png",
		},
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-PAGE123"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	assertPageContains := func(relPath string, fragments ...string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		got := string(content)
		for _, fragment := range fragments {
			if !strings.Contains(got, fragment) {
				t.Fatalf("Expected %s to contain %q, got:\n%s", relPath, fragment, got)
			}
		}
	}

	assertPageContains("index.html",
		`property="og:image" content="https://example.com/images/paged-og.png"`,
		`name="twitter:image" content="https://example.com/images/paged-og.png"`,
		`<link rel="canonical" href="https://example.com/">`,
		`googletagmanager.com/gtag/js?id=G-PAGE123`,
	)
	assertPageContains(filepath.Join("page", "2", "index.html"),
		`property="og:image" content="https://example.com/images/paged-og.png"`,
		`name="twitter:image" content="https://example.com/images/paged-og.png"`,
		`<link rel="canonical" href="https://example.com/page/2/">`,
		`googletagmanager.com/gtag/js?id=G-PAGE123`,
	)
}

func TestGenerate_OfficialWebsiteTheme_OutputsDoNotDisableCommentsOrAnalytics(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Outputs Integrated",
			Slug:        "outputs-integrated",
			Date:        time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Outputs integrated content.</p>",
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme outputs integration coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-OUT123"},
		},
		Comments: &config.CommentsConfig{
			Enabled:  true,
			Provider: "disqus",
			Disqus:   &config.DisqusConfig{Shortname: "gobin-out"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, relPath := range []string{"index.xml", "index.atom", "sitemap.xml", "robots.txt", "search-index.json", "search-index-min.json"} {
		if _, err := os.Stat(filepath.Join(outputDir, relPath)); !os.IsNotExist(err) {
			t.Fatalf("Expected %s to be absent when outputs disabled, got err=%v", relPath, err)
		}
	}

	indexHTML, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	if !strings.Contains(string(indexHTML), `googletagmanager.com/gtag/js?id=G-OUT123`) {
		t.Fatalf("Expected homepage analytics to remain enabled when outputs disabled, got:\n%s", string(indexHTML))
	}

	postHTML, err := os.ReadFile(filepath.Join(outputDir, "outputs-integrated", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read post page: %v", err)
	}
	postContent := string(postHTML)
	if !strings.Contains(postContent, `gobin-out.disqus.com/embed.js`) {
		t.Fatalf("Expected comments to remain enabled when outputs disabled, got:\n%s", postContent)
	}
	if !strings.Contains(postContent, `googletagmanager.com/gtag/js?id=G-OUT123`) {
		t.Fatalf("Expected analytics to remain enabled on post page when outputs disabled, got:\n%s", postContent)
	}
}

func TestGenerate_OfficialWebsiteTheme_AliasDoesNotAffectTaxonomyCanonicalLinks(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Alias Taxonomy Post",
			Slug:        "alias-taxonomy-post",
			Date:        time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
			Summary:     "Alias taxonomy summary.",
			SummaryHTML: "<p>Alias taxonomy summary.</p>",
			ContentHTML: "<p>Alias taxonomy content.</p>",
			Tags:        []string{"Alias Tag"},
			Categories:  []string{"Alias Category"},
			Aliases:     []string{"/legacy-alias/", "/archive/alias-taxonomy.html"},
		},
	}

	cfg := &config.Config{
		Title:        "Gobin Official",
		Description:  "Official theme alias taxonomy coverage.",
		BaseURL:      "https://example.com",
		Theme:        "official-website",
		ThemesDir:    "themes",
		StaticDir:    "assets",
		Paginate:     10,
		PaginatePath: "page",
		Permalinks: map[string]string{
			"posts": "/:year/:month/:day/:slug/",
		},
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	canonicalPath := "/2026/04/09/alias-taxonomy-post/"

	for _, relPath := range []string{
		filepath.Join("tags", "alias-tag", "index.html"),
		filepath.Join("categories", "alias-category", "index.html"),
	} {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		got := string(content)
		if !strings.Contains(got, `href="`+canonicalPath+`"`) {
			t.Fatalf("Expected %s to link to canonical permalink, got:\n%s", relPath, got)
		}
		if strings.Contains(got, "/legacy-alias/") || strings.Contains(got, "/archive/alias-taxonomy.html") {
			t.Fatalf("Expected %s to exclude alias paths, got:\n%s", relPath, got)
		}
	}

	aliasHTML, err := os.ReadFile(filepath.Join(outputDir, "legacy-alias", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read alias redirect page: %v", err)
	}
	aliasContent := string(aliasHTML)
	if !strings.Contains(aliasContent, `canonical" href="https://example.com`+canonicalPath+`"`) {
		t.Fatalf("Expected alias page to point canonical to main permalink, got:\n%s", aliasContent)
	}
}

func TestGenerate_OfficialWebsiteTheme_AliasConflictWithTaxonomyPage(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Alias Conflict Source",
			Slug:        "alias-conflict-source",
			Date:        time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC),
			Description: "Alias conflict description.",
			ContentHTML: "<p>Alias conflict content.</p>",
			Tags:        []string{"Conflict Tag"},
			Aliases:     []string{"/tags/conflict-tag/"},
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme alias conflict coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	err := Generate(posts, cfg, outputDir, false, false, true)
	if err == nil {
		t.Fatal("Expected alias conflict with taxonomy page to return an error")
	}
	if !strings.Contains(err.Error(), "conflicts with an existing generated page") {
		t.Fatalf("Expected existing page conflict error, got %v", err)
	}

	taxonomyOutput := filepath.Join(outputDir, "tags", "conflict-tag", "index.html")
	if _, statErr := os.Stat(taxonomyOutput); statErr != nil {
		t.Fatalf("Expected taxonomy page to remain generated despite alias conflict, got err=%v", statErr)
	}

	taxonomyHTML, readErr := os.ReadFile(taxonomyOutput)
	if readErr != nil {
		t.Fatalf("Failed to read taxonomy output: %v", readErr)
	}
	if !strings.Contains(string(taxonomyHTML), "Tag: conflict tag") {
		t.Fatalf("Expected taxonomy page content to remain intact, got:\n%s", string(taxonomyHTML))
	}
}

func TestGenerate_OfficialWebsiteTheme_SEOAndAnalyticsOnTaxonomyPages(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Taxonomy Integrated",
			Slug:        "taxonomy-integrated",
			Date:        time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
			Summary:     "Taxonomy summary.",
			SummaryHTML: "<p>Taxonomy summary.</p>",
			ContentHTML: "<p>Taxonomy content.</p>",
			Tags:        []string{"Tax Go"},
			Categories:  []string{"Tax Category"},
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme taxonomy integration coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		SEO: &config.SEOConfig{
			Image: "https://example.com/images/taxonomy-og.png",
		},
		Analytics: &config.AnalyticsConfig{
			Provider: "google",
			Google:   &config.GoogleAnalyticsConfig{TrackingID: "G-TAX123"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join("tags", "index.html"),
		filepath.Join("tags", "tax-go", "index.html"),
		filepath.Join("categories", "index.html"),
		filepath.Join("categories", "tax-category", "index.html"),
	} {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		got := string(content)
		if !strings.Contains(got, `property="og:image" content="https://example.com/images/taxonomy-og.png"`) {
			t.Fatalf("Expected %s to contain og:image, got:\n%s", relPath, got)
		}
		if !strings.Contains(got, `name="twitter:image" content="https://example.com/images/taxonomy-og.png"`) {
			t.Fatalf("Expected %s to contain twitter:image, got:\n%s", relPath, got)
		}
		if !strings.Contains(got, `googletagmanager.com/gtag/js?id=G-TAX123`) {
			t.Fatalf("Expected %s to contain analytics snippet, got:\n%s", relPath, got)
		}
	}
}

func TestGenerate_OfficialWebsiteTheme_CommentsNotRenderedOnListOrTaxonomyPages(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "No List Comments",
			Slug:        "no-list-comments",
			Date:        time.Date(2026, 4, 7, 11, 0, 0, 0, time.UTC),
			ContentHTML: "<p>No list comments content.</p>",
			Tags:        []string{"List Tag"},
			Categories:  []string{"List Category"},
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme comments scope coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Comments: &config.CommentsConfig{
			Enabled:  true,
			Provider: "disqus",
			Disqus:   &config.DisqusConfig{Shortname: "gobin-scope"},
		},
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, relPath := range []string{
		"index.html",
		filepath.Join("tags", "index.html"),
		filepath.Join("tags", "list-tag", "index.html"),
		filepath.Join("categories", "index.html"),
		filepath.Join("categories", "list-category", "index.html"),
	} {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		got := string(content)
		if strings.Contains(got, `id="disqus_thread"`) || strings.Contains(got, `.disqus.com/embed.js`) {
			t.Fatalf("Expected %s to exclude comments markup, got:\n%s", relPath, got)
		}
	}

	postContent, err := os.ReadFile(filepath.Join(outputDir, "no-list-comments", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read post page: %v", err)
	}
	if !strings.Contains(string(postContent), `gobin-scope.disqus.com/embed.js`) {
		t.Fatalf("Expected post page to retain comments markup, got:\n%s", string(postContent))
	}
}

func TestGenerate_OfficialWebsiteTheme_PaginationNavigationStructure(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{Title: "Nav One", Slug: "nav-one", Date: time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC), ContentHTML: "<p>One</p>"},
		{Title: "Nav Two", Slug: "nav-two", Date: time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC), ContentHTML: "<p>Two</p>"},
		{Title: "Nav Three", Slug: "nav-three", Date: time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC), ContentHTML: "<p>Three</p>"},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme pagination navigation coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      1,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	indexContent, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("Failed to read index page: %v", err)
	}
	if !strings.Contains(string(indexContent), `href="/page/2/"`) {
		t.Fatalf("Expected first page to link to older posts page, got:\n%s", string(indexContent))
	}

	page2Content, err := os.ReadFile(filepath.Join(outputDir, "page", "2", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read page 2: %v", err)
	}
	page2 := string(page2Content)
	if !strings.Contains(page2, `href="/"`) {
		t.Fatalf("Expected page 2 to link back to first page, got:\n%s", page2)
	}
	if !strings.Contains(page2, `href="/page/3/"`) {
		t.Fatalf("Expected page 2 to link to page 3, got:\n%s", page2)
	}
}

func TestGenerate_IndexPageMetadataTitles(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createGoldenTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create golden test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{Title: "Alpha Post", Slug: "alpha-post", Date: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC), ContentHTML: "<p>Alpha</p>"},
		{Title: "Beta Post", Slug: "beta-post", Date: time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC), ContentHTML: "<p>Beta</p>"},
	}

	cfg := &config.Config{
		Title:        "Golden Blog",
		Description:  "Golden site description.",
		BaseURL:      "https://example.com",
		StaticDir:    "assets",
		ThemesDir:    "themes",
		Paginate:     1,
		PaginatePath: "page",
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	indexContent, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("Failed to read index page: %v", err)
	}
	indexHTML := string(indexContent)
	if !strings.Contains(indexHTML, `<title>Golden Blog</title>`) {
		t.Fatalf("Expected homepage title to avoid duplicated site title, got:\n%s", indexHTML)
	}
	if !strings.Contains(indexHTML, `property="og:title" content="Golden Blog"`) {
		t.Fatalf("Expected homepage og:title to use site title, got:\n%s", indexHTML)
	}
	if !strings.Contains(indexHTML, `name="twitter:title" content="Golden Blog"`) {
		t.Fatalf("Expected homepage twitter:title to use site title, got:\n%s", indexHTML)
	}

	page2Content, err := os.ReadFile(filepath.Join(outputDir, "page", "2", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read paginated index page: %v", err)
	}
	page2HTML := string(page2Content)
	if !strings.Contains(page2HTML, `<title>Page 2 - Golden Blog</title>`) {
		t.Fatalf("Expected paginated page title to include page number, got:\n%s", page2HTML)
	}
	if !strings.Contains(page2HTML, `property="og:title" content="Page 2"`) {
		t.Fatalf("Expected paginated page og:title to include page number, got:\n%s", page2HTML)
	}
	if !strings.Contains(page2HTML, `name="twitter:title" content="Page 2"`) {
		t.Fatalf("Expected paginated page twitter:title to include page number, got:\n%s", page2HTML)
	}
}

func TestGenerate_OfficialWebsiteTheme_EmptySite(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme empty site coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(nil, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	indexContent := string(indexHTML)
	if !strings.Contains(indexContent, `Get Started`) {
		t.Fatalf("Expected empty theme homepage to render hero content, got:\n%s", indexContent)
	}
	if strings.Contains(indexContent, `href="/page/2/"`) {
		t.Fatalf("Expected empty theme homepage to omit pagination links, got:\n%s", indexContent)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "page", "2", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Expected no extra pagination page for empty site, got err=%v", err)
	}
}

func TestGenerate_OfficialWebsiteTheme_EmptyTaxonomyIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme empty taxonomy coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(nil, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	checks := map[string]string{
		filepath.Join("tags", "index.html"):       `No tags found.`,
		filepath.Join("categories", "index.html"): `No categories found.`,
	}

	for relPath, expected := range checks {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		if !strings.Contains(string(content), expected) {
			t.Fatalf("Expected %s to contain %q, got:\n%s", relPath, expected, string(content))
		}
	}
}

func TestGenerate_OfficialWebsiteTheme_EmptySiteStillGenerates404(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme empty 404 coverage.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(nil, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	notFoundHTML, err := os.ReadFile(filepath.Join(outputDir, "404.html"))
	if err != nil {
		t.Fatalf("Failed to read 404.html: %v", err)
	}
	if !strings.Contains(string(notFoundHTML), `Page Not Found`) {
		t.Fatalf("Expected empty site to still generate 404 page, got:\n%s", string(notFoundHTML))
	}
}

func TestGenerate_OutputsConfigMatrix(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createGoldenTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Matrix Post",
			Slug:        "matrix-post",
			Date:        time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC),
			Summary:     "Matrix summary.",
			Content:     "Matrix content.",
			ContentHTML: "<p>Matrix content.</p>",
			Tags:        []string{"Go"},
			Categories:  []string{"Tech"},
		},
	}

	cfg := &config.Config{
		Title:           "Matrix Blog",
		Description:     "Matrix config test.",
		Author:          "Matrix Author",
		BaseURL:         "https://example.com",
		StaticDir:       "assets",
		ThemesDir:       "themes",
		Paginate:        10,
		PaginatePath:    "page",
		EnableRobotsTXT: true,
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(true),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	expectedPresent := []string{
		"index.html",
		"404.html",
		"matrix-post/index.html",
		"search-index.json",
		"search-index-min.json",
	}
	for _, relPath := range expectedPresent {
		if _, err := os.Stat(filepath.Join(outputDir, relPath)); err != nil {
			t.Fatalf("Expected %s to exist, got error: %v", relPath, err)
		}
	}

	expectedAbsent := []string{
		"index.xml",
		"index.atom",
		"sitemap.xml",
		"robots.txt",
	}
	for _, relPath := range expectedAbsent {
		if _, err := os.Stat(filepath.Join(outputDir, relPath)); !os.IsNotExist(err) {
			t.Fatalf("Expected %s to be absent, got err=%v", relPath, err)
		}
	}
}

func TestGenerate_OfficialWebsiteThemeWithAliases(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Theme Alias",
			Slug:        "theme-alias",
			Date:        time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			Description: "Theme alias description.",
			ContentHTML: "<p>Theme alias content.</p>",
			Aliases:     []string{"/old-theme-alias/", "/legacy/theme-alias.html"},
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme alias test.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	aliasChecks := map[string]string{
		"old-theme-alias/index.html": "/theme-alias/",
		"legacy/theme-alias.html":    "/theme-alias/",
		"theme-alias/index.html":     "Theme alias content.",
	}

	for relPath, expected := range aliasChecks {
		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", relPath, err)
		}
		if !strings.Contains(string(content), expected) {
			t.Fatalf("Expected %s to contain %q, got:\n%s", relPath, expected, string(content))
		}
	}

	aliasPage, err := os.ReadFile(filepath.Join(outputDir, "old-theme-alias", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read alias page: %v", err)
	}
	if !strings.Contains(string(aliasPage), `canonical" href="https://example.com/theme-alias/"`) {
		t.Fatalf("Expected alias page to contain canonical target URL, got:\n%s", string(aliasPage))
	}
}

func TestGenerate_OfficialWebsiteThemeDraftFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	if err := createOfficialThemeTestSite(siteDir); err != nil {
		t.Fatalf("Failed to create official theme test site: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(oldWd)

	posts := []*parser.Post{
		{
			Title:       "Visible Theme Post",
			Slug:        "visible-theme-post",
			Date:        time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Visible theme content.</p>",
		},
		{
			Title:       "Draft Theme Post",
			Slug:        "draft-theme-post",
			Date:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Draft theme content.</p>",
			Draft:       true,
		},
		{
			Title:       "Hidden Theme Post",
			Slug:        "hidden-theme-post",
			Date:        time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
			ContentHTML: "<p>Hidden theme content.</p>",
			Published:   boolPtr(false),
		},
	}

	cfg := &config.Config{
		Title:         "Gobin Official",
		Description:   "Official theme draft test.",
		BaseURL:       "https://example.com",
		Theme:         "official-website",
		ThemesDir:     "themes",
		StaticDir:     "assets",
		Paginate:      10,
		PaginatePath:  "page",
		RepositoryURL: "https://github.com/mengbin92/gobin",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(true),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	if err := Generate(posts, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate without drafts failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "visible-theme-post", "index.html")); err != nil {
		t.Fatalf("Expected visible post to exist, got error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "draft-theme-post", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Expected draft post to be absent by default, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "hidden-theme-post", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Expected published=false post to be absent by default, got err=%v", err)
	}

	searchIndex, err := os.ReadFile(filepath.Join(outputDir, "search-index.json"))
	if err != nil {
		t.Fatalf("Failed to read search index: %v", err)
	}
	if strings.Contains(string(searchIndex), "Draft Theme Post") || strings.Contains(string(searchIndex), "Hidden Theme Post") {
		t.Fatalf("Expected hidden posts to be excluded from default search index, got:\n%s", string(searchIndex))
	}

	if err := Generate(posts, cfg, outputDir, false, true, true); err != nil {
		t.Fatalf("Generate with drafts failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "draft-theme-post", "index.html")); err != nil {
		t.Fatalf("Expected draft post to exist when drafts enabled, got error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "hidden-theme-post", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Expected published=false post to stay absent even with drafts enabled, got err=%v", err)
	}

	searchIndexWithDrafts, err := os.ReadFile(filepath.Join(outputDir, "search-index.json"))
	if err != nil {
		t.Fatalf("Failed to read search index with drafts: %v", err)
	}
	if !strings.Contains(string(searchIndexWithDrafts), "Draft Theme Post") {
		t.Fatalf("Expected draft post to appear in search index when drafts enabled, got:\n%s", string(searchIndexWithDrafts))
	}
	if strings.Contains(string(searchIndexWithDrafts), "Hidden Theme Post") {
		t.Fatalf("Expected published=false post to remain excluded from search index, got:\n%s", string(searchIndexWithDrafts))
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
	partialsDir := filepath.Join(tmpDir, "templates", "partials")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(partialsDir, 0755)

	baseTemplate := `{{ define "base" }}{{ render .HeaderTemplate . }}{{ render .MainTemplate . }}{{ render .FooterTemplate . }}{{ end }}`
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

	os.WriteFile(filepath.Join(templatesDir, "base.html"), []byte(baseTemplate), 0644)
	os.WriteFile(filepath.Join(templatesDir, "single.html"), []byte(singleTemplate), 0644)
	os.WriteFile(filepath.Join(templatesDir, "list.html"), []byte(`{{ define "listPage" }}list{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "404.html"), []byte(`{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}404{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "header.html"), []byte(`{{ define "header" }}h{{ end }}{{ define "headerNested" }}h{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "footer.html"), []byte(`{{ define "footer" }}f{{ end }}{{ define "footerNested" }}f{{ end }}`), 0644)

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

func TestGenerateAliasPages(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	post := &parser.Post{
		Title: "Alias Post",
		URL:   "/posts/alias-post/",
		Aliases: []string{
			"/old-url/",
			"legacy/path",
			"/archive/post.html",
		},
	}

	if err := generateAliasPages([]*parser.Post{post}, &config.Config{BaseURL: "https://example.com"}, outputDir); err != nil {
		t.Fatalf("generateAliasPages failed: %v", err)
	}

	assertAliasFile := func(relPath, expectedTarget string) {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(outputDir, relPath))
		if err != nil {
			t.Fatalf("Failed to read alias output %s: %v", relPath, err)
		}

		html := string(content)
		if !strings.Contains(html, `http-equiv="refresh" content="0; url=`+expectedTarget+`"`) {
			t.Fatalf("Expected alias output %s to redirect to %s, got %s", relPath, expectedTarget, html)
		}
		if !strings.Contains(html, `<link rel="canonical" href="https://example.com/posts/alias-post/">`) {
			t.Fatalf("Expected alias output %s to contain canonical target URL", relPath)
		}
	}

	assertAliasFile(filepath.Join("old-url", "index.html"), "/posts/alias-post/")
	assertAliasFile(filepath.Join("legacy", "path", "index.html"), "/posts/alias-post/")
	assertAliasFile(filepath.Join("archive", "post.html"), "/posts/alias-post/")
}

func TestGenerateAliasPages_Conflict(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(filepath.Join(outputDir, "existing"), 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "existing", "index.html"), []byte("occupied"), 0644); err != nil {
		t.Fatalf("Failed to seed conflicting output: %v", err)
	}

	post := &parser.Post{
		Title:   "Alias Conflict",
		URL:     "/posts/conflict/",
		Aliases: []string{"/existing/"},
	}

	err := generateAliasPages([]*parser.Post{post}, &config.Config{}, outputDir)
	if err == nil {
		t.Fatal("Expected alias conflict to return an error")
	}
	if !strings.Contains(err.Error(), "conflicts with an existing generated page") {
		t.Fatalf("Expected conflict error, got %v", err)
	}
}

func TestGenerateTaxonomyPages_UsesTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	templatesDir := filepath.Join(tmpDir, "templates", "_default")
	partialsDir := filepath.Join(tmpDir, "templates", "partials")

	os.MkdirAll(outputDir, 0755)
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(partialsDir, 0755)

	os.WriteFile(filepath.Join(templatesDir, "base.html"), []byte(`{{ define "base" }}{{ render .HeaderTemplate . }}{{ render .MainTemplate . }}{{ render .FooterTemplate . }}{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "single.html"), []byte(`{{ define "singlePage" }}single{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "list.html"), []byte(`{{ define "listPage" }}list{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "404.html"), []byte(`{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}404{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "taxonomy.html"), []byte(`
{{ define "taxonomyTermsPage" }}<html><body>{{ template "header" . }}<h1>{{ .Title }}</h1>{{ range .Terms }}<span>{{ .Name }}</span>{{ end }}{{ template "footer" . }}</body></html>{{ end }}
{{ define "taxonomyPage" }}<html><body>{{ template "header" . }}<h1>{{ .Title }}</h1>{{ range .Posts }}<article>{{ .Title }}</article>{{ end }}{{ template "footer" . }}</body></html>{{ end }}
`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "header.html"), []byte(`{{ define "header" }}<header>shared-header</header>{{ end }}{{ define "headerNested" }}<header>shared-header</header>{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "footer.html"), []byte(`{{ define "footer" }}<footer>shared-footer</footer>{{ end }}{{ define "footerNested" }}<footer>shared-footer</footer>{{ end }}`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{Title: "Test Blog"}
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	posts := []*parser.Post{
		{Title: "Go Tips", Slug: "go-tips", URL: "/go-tips/", Date: time.Now(), Tags: []string{"Go"}, Categories: []string{"Tech"}},
	}

	if _, _, err := generateTaxonomyPages(posts, cfg, outputDir, tmpl); err != nil {
		t.Fatalf("generateTaxonomyPages failed: %v", err)
	}

	tagPageContent, err := os.ReadFile(filepath.Join(outputDir, "tags", "go", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read generated tag page: %v", err)
	}
	if !strings.Contains(string(tagPageContent), "shared-header") || !strings.Contains(string(tagPageContent), "shared-footer") {
		t.Fatal("Expected taxonomy page to be rendered through shared templates")
	}

	categoryIndexContent, err := os.ReadFile(filepath.Join(outputDir, "categories", "index.html"))
	if err != nil {
		t.Fatalf("Failed to read generated category index page: %v", err)
	}
	if !strings.Contains(string(categoryIndexContent), "Categories") {
		t.Fatal("Expected taxonomy terms page to contain template title")
	}
}

func TestGenerateNotFoundPage(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	templatesDir := filepath.Join(tmpDir, "templates", "_default")
	partialsDir := filepath.Join(tmpDir, "templates", "partials")

	os.MkdirAll(outputDir, 0755)
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(partialsDir, 0755)

	os.WriteFile(filepath.Join(templatesDir, "base.html"), []byte(`{{ define "base" }}{{ render .HeaderTemplate . }}{{ render .MainTemplate . }}{{ render .FooterTemplate . }}{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "single.html"), []byte(`{{ define "singlePage" }}single{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "list.html"), []byte(`{{ define "listPage" }}list{{ end }}`), 0644)
	os.WriteFile(filepath.Join(templatesDir, "404.html"), []byte(`{{ define "notFoundMain" }}<section>missing page</section>{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "header.html"), []byte(`{{ define "header" }}<header>header</header>{{ end }}{{ define "headerNested" }}<header>nested</header>{{ end }}`), 0644)
	os.WriteFile(filepath.Join(partialsDir, "footer.html"), []byte(`{{ define "footer" }}<footer>footer</footer>{{ end }}{{ define "footerNested" }}<footer>nested-footer</footer>{{ end }}`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := &config.Config{Title: "Test Blog", BaseURL: "https://example.com"}
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}

	if err := generateNotFoundPage(cfg, outputDir, tmpl); err != nil {
		t.Fatalf("generateNotFoundPage failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outputDir, "404.html"))
	if err != nil {
		t.Fatalf("Failed to read generated 404 page: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "missing page") || !strings.Contains(got, "header") || !strings.Contains(got, "footer") {
		t.Fatal("Expected generated 404 page to render through templates")
	}
}

func TestGenerateRobotsTXT(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		BaseURL:         "https://example.com",
		EnableRobotsTXT: true,
	}

	if err := generateRobotsTXT(cfg, tmpDir); err != nil {
		t.Fatalf("generateRobotsTXT failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "robots.txt"))
	if err != nil {
		t.Fatalf("Failed to read robots.txt: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "User-agent: *") {
		t.Fatal("Expected robots.txt to contain user-agent rule")
	}
	if !strings.Contains(got, "Sitemap: https://example.com/sitemap.xml") {
		t.Fatal("Expected robots.txt to contain sitemap URL")
	}
}

func TestGenerateRobotsTXT_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		BaseURL:         "https://example.com",
		EnableRobotsTXT: false,
	}

	if err := generateRobotsTXT(cfg, tmpDir); err != nil {
		t.Fatalf("generateRobotsTXT failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "robots.txt")); !os.IsNotExist(err) {
		t.Fatal("Expected robots.txt to be skipped when disabled")
	}
}

func TestGenerateRobotsTXT_RelativeBaseURLOmitsSitemap(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		BaseURL:         "/",
		EnableRobotsTXT: true,
	}

	if err := generateRobotsTXT(cfg, tmpDir); err != nil {
		t.Fatalf("generateRobotsTXT failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "robots.txt"))
	if err != nil {
		t.Fatalf("Failed to read robots.txt: %v", err)
	}

	if strings.Contains(string(content), "Sitemap:") {
		t.Fatal("Expected robots.txt to omit sitemap line for relative baseURL")
	}
}

func TestHasAbsoluteBaseURL(t *testing.T) {
	if !hasAbsoluteBaseURL("https://example.com") {
		t.Fatal("Expected absolute https URL to be recognized")
	}
	if hasAbsoluteBaseURL("/") {
		t.Fatal("Expected relative baseURL to be rejected")
	}
	if hasAbsoluteBaseURL("example.com") {
		t.Fatal("Expected host without scheme to be rejected")
	}
}

func TestOutputEnabled(t *testing.T) {
	cfg := &config.Config{
		Outputs: &config.OutputsConfig{
			Feed:   boolPtr(false),
			Search: boolPtr(true),
		},
	}

	if outputEnabled(cfg, "feed", true) {
		t.Fatal("Expected explicit outputs.feed=false to disable feed")
	}
	if !outputEnabled(cfg, "search", false) {
		t.Fatal("Expected explicit outputs.search=true to enable search")
	}
	if !outputEnabled(cfg, "sitemap", true) {
		t.Fatal("Expected unset outputs.sitemap to use fallback=true")
	}
}

func TestCleanOutputDir_RemovesStaleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "public")

	if err := os.MkdirAll(filepath.Join(outputDir, "old-dir"), 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "stale.txt"), []byte("stale"), 0644); err != nil {
		t.Fatalf("Failed to create stale file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "old-dir", "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	if err := cleanOutputDir(outputDir); err != nil {
		t.Fatalf("cleanOutputDir failed: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("Failed to read output dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatal("Expected output directory to be emptied")
	}
}

func TestCleanOutputDir_RefusesWorkingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := cleanOutputDir("."); err == nil {
		t.Fatal("Expected cleaning current working directory to be rejected")
	}
}

func TestGenerate_ReplacesJekyllSiteVariablesInContent(t *testing.T) {
	siteDir := t.TempDir()
	mustWriteFile(t, filepath.Join(siteDir, "config.yaml"), `title: Test
author: Test
baseURL: https://example.com
contentDir: _posts
publishDir: public
`)
	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2024-01-01-test.md"), `---
title: "Replace Vars"
date: 2024-01-01T00:00:00Z
---

<div><img src="{{site.url}}/images/test.png"></div>`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}{{ safeHTML .Post.ContentHTML }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "list.html"), `{{ define "listMain" }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "header.html"), `{{ define "header" }}{{ end }}{{ define "headerNested" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "footer.html"), `{{ define "footer" }}{{ end }}{{ define "footerNested" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "comments.html"), `{{ define "comments" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "analytics.html"), `{{ define "analytics" }}{{ end }}`)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cfg, err := config.Load("config.yaml")
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	posts, err := parser.ParsePosts("_posts")
	if err != nil {
		t.Fatalf("parse posts failed: %v", err)
	}

	if err := Generate(posts, cfg, "public", false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	output, err := os.ReadFile(filepath.Join(siteDir, "public", "test", "index.html"))
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}

	if strings.Contains(string(output), "{{site.url}}") {
		t.Fatalf("expected site.url placeholder to be replaced, got %s", string(output))
	}
	if !strings.Contains(string(output), `https://example.com/images/test.png`) {
		t.Fatalf("expected absolute image URL, got %s", string(output))
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", path, err)
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
