package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig tests loading a valid config file
func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
author: "Test Author"
languageCode: "zh-CN"
baseURL: "https://example.com"
timezone: "Asia/Shanghai"

# Directory configuration
contentDir: "_posts"
staticDir: "assets"
publishDir: "public"
themesDir: "themes"

# Theme configuration
theme: "default"

# Pagination
paginate: 10
paginatePath: "page"

# Permalink configuration
permalinks:
  posts: "/:year/:month/:day/:title/"

# Navigation
navbarLinks:
  - name: "首页"
    url: "/"
  - name: "关于"
    url: "/about/"

# Social media
social:
  github: "testuser"
  email: "test@example.com"

# Feature flags
enableEmoji: true
enableGitInfo: true
enableRobotsTXT: true

# SEO configuration
seo:
  enabled: true
  openGraph: true
  twitterCard: true

# Comments configuration
comments:
  enabled: true
  provider: "utterances"

# Analytics configuration
analytics:
  provider: "google"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "themes", "default"), 0755); err != nil {
		t.Fatalf("Failed to create test theme directory: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify basic configuration
	if cfg.Title != "Test Blog" {
		t.Errorf("Expected title 'Test Blog', got '%s'", cfg.Title)
	}
	if cfg.Description != "Test blog description" {
		t.Errorf("Expected description 'Test blog description', got '%s'", cfg.Description)
	}
	if cfg.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", cfg.Author)
	}
	if cfg.LanguageCode != "zh-CN" {
		t.Errorf("Expected language 'zh-CN', got '%s'", cfg.LanguageCode)
	}
	if cfg.BaseURL != "https://example.com" {
		t.Errorf("Expected baseURL 'https://example.com', got '%s'", cfg.BaseURL)
	}

	// Verify directory configuration
	if cfg.ContentDir != "_posts" {
		t.Errorf("Expected contentDir '_posts', got '%s'", cfg.ContentDir)
	}
	if cfg.PageDir != "pages" {
		t.Errorf("Expected pageDir 'pages', got '%s'", cfg.PageDir)
	}
	if cfg.StaticDir != "assets" {
		t.Errorf("Expected staticDir 'assets', got '%s'", cfg.StaticDir)
	}
	if cfg.PublishDir != "public" {
		t.Errorf("Expected publishDir 'public', got '%s'", cfg.PublishDir)
	}
	if cfg.ThemesDir != "themes" {
		t.Errorf("Expected themesDir 'themes', got '%s'", cfg.ThemesDir)
	}

	// Verify pagination
	if cfg.Paginate != 10 {
		t.Errorf("Expected paginate 10, got %d", cfg.Paginate)
	}
	if cfg.PaginatePath != "page" {
		t.Errorf("Expected paginatePath 'page', got '%s'", cfg.PaginatePath)
	}

	// Verify navigation
	if len(cfg.NavbarLinks) != 2 {
		t.Errorf("Expected 2 navigation links, got %d", len(cfg.NavbarLinks))
	}
	if cfg.NavbarLinks[0].Name != "首页" {
		t.Errorf("Expected first nav link name '首页', got '%s'", cfg.NavbarLinks[0].Name)
	}

	// Verify social media
	if cfg.Social["github"] != "testuser" {
		t.Errorf("Expected github 'testuser', got '%s'", cfg.Social["github"])
	}

	// Verify feature flags
	if !cfg.EnableEmoji {
		t.Error("Expected enableEmoji to be true")
	}
	if !cfg.EnableGitInfo {
		t.Error("Expected enableGitInfo to be true")
	}
	if !cfg.EnableRobotsTXT {
		t.Error("Expected enableRobotsTXT to be true")
	}

	// Verify SEO configuration
	if cfg.SEO == nil {
		t.Fatal("Expected SEO config to be loaded")
	}
	if !cfg.SEO.Enabled {
		t.Error("Expected SEO to be enabled")
	}

	// Verify comments configuration
	if cfg.Comments == nil {
		t.Fatal("Expected Comments config to be loaded")
	}
	if !cfg.Comments.Enabled {
		t.Error("Expected Comments to be enabled")
	}
	if cfg.Comments.Provider != "utterances" {
		t.Errorf("Expected provider 'utterances', got '%s'", cfg.Comments.Provider)
	}

	// Verify analytics configuration
	if cfg.Analytics == nil {
		t.Fatal("Expected Analytics config to be loaded")
	}
	if cfg.Analytics.Provider != "google" {
		t.Errorf("Expected analytics provider 'google', got '%s'", cfg.Analytics.Provider)
	}
}

func TestLoadConfig_MarkupAllowUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Markup Test"
baseURL: "https://example.com"
markup:
  allowUnsafeHTML: false
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Markup == nil || cfg.Markup.AllowUnsafeHTML == nil {
		t.Fatalf("Expected markup.allowUnsafeHTML to be loaded")
	}
	if *cfg.Markup.AllowUnsafeHTML {
		t.Fatal("Expected allowUnsafeHTML=false")
	}
}

// TestLoadConfig_WithDefaults tests that default values are set
func TestLoadConfig_WithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	// Minimal config without optional fields
	configContent := `title: "Minimal Blog"
description: "Minimal description"
baseURL: "https://example.com"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify defaults
	if cfg.Paginate != 10 {
		t.Errorf("Expected default paginate 10, got %d", cfg.Paginate)
	}
	if cfg.PaginatePath != "page" {
		t.Errorf("Expected default paginatePath 'page', got '%s'", cfg.PaginatePath)
	}
	if cfg.ContentDir != "_posts" {
		t.Errorf("Expected default contentDir '_posts', got '%s'", cfg.ContentDir)
	}
	if cfg.PageDir != "pages" {
		t.Errorf("Expected default pageDir 'pages', got '%s'", cfg.PageDir)
	}
	if cfg.PublishDir != "public" {
		t.Errorf("Expected default publishDir 'public', got '%s'", cfg.PublishDir)
	}
	if cfg.StaticDir != "assets" {
		t.Errorf("Expected default staticDir 'assets', got '%s'", cfg.StaticDir)
	}
	if cfg.ThemesDir != "themes" {
		t.Errorf("Expected default themesDir 'themes', got '%s'", cfg.ThemesDir)
	}
}

func TestLoadDefault_PrefersConfigYaml(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.WriteFile("config.yaml", []byte("title: Config YAML\nbaseURL: https://example.com\n"), 0644); err != nil {
		t.Fatalf("Failed to write config.yaml: %v", err)
	}
	if err := os.WriteFile("_config.yml", []byte("title: Jekyll Config\nbaseURL: https://example.com\n"), 0644); err != nil {
		t.Fatalf("Failed to write _config.yml: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}
	if cfg.Title != "Config YAML" {
		t.Fatalf("Expected config.yaml to win, got %q", cfg.Title)
	}
}

func TestLoadDefault_FallsBackToJekyllConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.WriteFile("_config.yml", []byte("title: Jekyll Config\nbaseURL: https://example.com\n"), 0644); err != nil {
		t.Fatalf("Failed to write _config.yml: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}
	if cfg.Title != "Jekyll Config" {
		t.Fatalf("Expected _config.yml fallback, got %q", cfg.Title)
	}
}

func TestLoadDefault_NoConfigFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if _, err := LoadDefault(); err == nil {
		t.Fatal("Expected LoadDefault to fail when no config file exists")
	}
}

func TestNormalize_AppliesDefaultsToNilConfig(t *testing.T) {
	cfg := Normalize(nil)

	if cfg.Paginate != 10 {
		t.Fatalf("Expected default paginate 10, got %d", cfg.Paginate)
	}
	if cfg.PaginatePath != "page" {
		t.Fatalf("Expected default paginatePath 'page', got %q", cfg.PaginatePath)
	}
	if cfg.ContentDir != "_posts" {
		t.Fatalf("Expected default contentDir '_posts', got %q", cfg.ContentDir)
	}
	if cfg.PageDir != "pages" {
		t.Fatalf("Expected default pageDir 'pages', got %q", cfg.PageDir)
	}
	if cfg.PublishDir != "public" {
		t.Fatalf("Expected default publishDir 'public', got %q", cfg.PublishDir)
	}
	if cfg.StaticDir != "assets" {
		t.Fatalf("Expected default staticDir 'assets', got %q", cfg.StaticDir)
	}
	if cfg.ThemesDir != "themes" {
		t.Fatalf("Expected default themesDir 'themes', got %q", cfg.ThemesDir)
	}
}

func TestLoadConfig_WithOutputs(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Outputs Blog"
description: "Outputs config"
baseURL: "https://example.com"
outputs:
  feed: false
  search: true
  sitemap: false
  robots: true
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Outputs == nil {
		t.Fatal("Expected outputs config to be loaded")
	}
	if cfg.Outputs.Feed == nil || *cfg.Outputs.Feed {
		t.Error("Expected outputs.feed to be false")
	}
	if cfg.Outputs.Search == nil || !*cfg.Outputs.Search {
		t.Error("Expected outputs.search to be true")
	}
	if cfg.Outputs.Sitemap == nil || *cfg.Outputs.Sitemap {
		t.Error("Expected outputs.sitemap to be false")
	}
	if cfg.Outputs.Robots == nil || !*cfg.Outputs.Robots {
		t.Error("Expected outputs.robots to be true")
	}
}

// TestLoadConfig_NonExistentFile tests error handling for missing file
func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

// TestLoadConfig_InvalidYAML tests error handling for invalid YAML
func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	invalidYAML := `
title: "Test Blog"
description: "Test blog description"
this is not valid YAML: {[
baseURL: "https://example.com"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

// TestLoadConfig_EmptyPaginate tests zero value for paginate
func TestLoadConfig_EmptyPaginate(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
paginate: 0
baseURL: "https://example.com"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should be set to default
	if cfg.Paginate != 10 {
		t.Errorf("Expected default paginate 10 for zero value, got %d", cfg.Paginate)
	}
}

// TestNavbarLink structure test
func TestNavbarLink(t *testing.T) {
	link := NavbarLink{
		Name: "Test Link",
		URL:  "/test/",
	}

	if link.Name != "Test Link" {
		t.Errorf("Expected name 'Test Link', got '%s'", link.Name)
	}
	if link.URL != "/test/" {
		t.Errorf("Expected URL '/test/', got '%s'", link.URL)
	}
}

// TestNavbarLink_Empty tests empty navbar links
func TestNavbarLink_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
baseURL: "https://example.com"
navbarLinks: []
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.NavbarLinks) != 0 {
		t.Errorf("Expected 0 navigation links, got %d", len(cfg.NavbarLinks))
	}
}

// TestSEOConfig tests SEO configuration parsing
func TestSEOConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
baseURL: "https://example.com"
seo:
  enabled: true
  openGraph: true
  twitterCard: true
  image: "/images/og-image.png"
  twitterUsername: "@testuser"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.SEO == nil {
		t.Fatal("Expected SEO config to be loaded")
	}
	if !cfg.SEO.Enabled {
		t.Error("Expected SEO to be enabled")
	}
	if !cfg.SEO.OpenGraph {
		t.Error("Expected OpenGraph to be enabled")
	}
	if !cfg.SEO.TwitterCard {
		t.Error("Expected TwitterCard to be enabled")
	}
	if cfg.SEO.Image != "/images/og-image.png" {
		t.Errorf("Expected image '/images/og-image.png', got '%s'", cfg.SEO.Image)
	}
	if cfg.SEO.TwitterUsername != "@testuser" {
		t.Errorf("Expected twitterUsername '@testuser', got '%s'", cfg.SEO.TwitterUsername)
	}
}

// TestCommentsConfig tests comments configuration parsing
func TestCommentsConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test Utterances configuration
	configContent := `title: "Test Blog"
description: "Test blog description"
baseURL: "https://example.com"
comments:
  enabled: true
  provider: "utterances"
  utterances:
    repo: "testuser/testrepo"
    theme: "github-light"
    label: "blog-comments"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Comments == nil {
		t.Fatal("Expected Comments config to be loaded")
	}
	if !cfg.Comments.Enabled {
		t.Error("Expected Comments to be enabled")
	}
	if cfg.Comments.Provider != "utterances" {
		t.Errorf("Expected provider 'utterances', got '%s'", cfg.Comments.Provider)
	}
	if cfg.Comments.Utterances == nil {
		t.Fatal("Expected Utterances config to be loaded")
	}
	if cfg.Comments.Utterances.Repo != "testuser/testrepo" {
		t.Errorf("Expected repo 'testuser/testrepo', got '%s'", cfg.Comments.Utterances.Repo)
	}
}

// TestAnalyticsConfig tests analytics configuration parsing
func TestAnalyticsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
baseURL: "https://example.com"
analytics:
  provider: "google"
  google:
    trackingID: "UA-12345678-1"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Analytics == nil {
		t.Fatal("Expected Analytics config to be loaded")
	}
	if cfg.Analytics.Provider != "google" {
		t.Errorf("Expected provider 'google', got '%s'", cfg.Analytics.Provider)
	}
	if cfg.Analytics.Google == nil {
		t.Fatal("Expected Google Analytics config to be loaded")
	}
	if cfg.Analytics.Google.TrackingID != "UA-12345678-1" {
		t.Errorf("Expected trackingID 'UA-12345678-1', got '%s'", cfg.Analytics.Google.TrackingID)
	}
}

// BenchmarkLoadConfig benchmarks config loading
func BenchmarkLoadConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configContent := `title: "Test Blog"
description: "Test blog description"
author: "Test Author"
languageCode: "zh-CN"
baseURL: "https://example.com"
paginate: 10
navbarLinks:
  - name: "首页"
    url: "/"
social:
  github: "testuser"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatalf("Failed to create test config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load(configPath)
		if err != nil {
			b.Fatalf("Load failed: %v", err)
		}
	}
}
