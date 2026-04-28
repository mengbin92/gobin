package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeSite_CreatesScaffold(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "my-blog")

	var stdout bytes.Buffer
	if err := initializeSite(&stdout, siteDir); err != nil {
		t.Fatalf("initializeSite failed: %v", err)
	}

	requiredPaths := []string{
		filepath.Join(siteDir, "config.yaml"),
		filepath.Join(siteDir, ".gitignore"),
		filepath.Join(siteDir, "assets", "css", "main.css"),
		filepath.Join(siteDir, "templates", "_default", "base.html"),
		filepath.Join(siteDir, "templates", "_default", "single.html"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Expected scaffold file %s to exist: %v", path, err)
		}
	}

	postMatches, err := filepath.Glob(filepath.Join(siteDir, "_posts", "*-welcome.md"))
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(postMatches) != 1 {
		t.Fatalf("Expected one sample post, got %d (%#v)", len(postMatches), postMatches)
	}

	output := stdout.String()
	if !strings.Contains(output, "Initializing new blog site in:") {
		t.Fatalf("Expected init output to mention target dir, got %q", output)
	}
	if !strings.Contains(output, "Site initialized successfully!") {
		t.Fatalf("Expected init output to confirm success, got %q", output)
	}
}

func TestRunBuild_GeneratesSite(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")

	if err := initializeSite(io.Discard, siteDir); err != nil {
		t.Fatalf("initializeSite failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	var stdout bytes.Buffer
	if err := runBuild(&stdout, false, false, true); err != nil {
		t.Fatalf("runBuild failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Found 1 posts") {
		t.Fatalf("Expected build output to include post count, got %q", output)
	}
	if !strings.Contains(output, "Static assets:") {
		t.Fatalf("Expected build output to include static asset stats, got %q", output)
	}
	if !strings.Contains(output, "Site generated successfully in 'public' directory") {
		t.Fatalf("Expected build output to confirm publish dir, got %q", output)
	}

	generatedPaths := []string{
		filepath.Join(siteDir, "public", "index.html"),
		filepath.Join(siteDir, "public", "404.html"),
		filepath.Join(siteDir, "public", "search-index.json"),
	}
	for _, path := range generatedPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Expected generated file %s to exist: %v", path, err)
		}
	}
}

func TestRunBuild_ReturnsConfigError(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	err := runBuild(io.Discard, false, false, true)
	if err == nil {
		t.Fatal("Expected runBuild to fail without config")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Fatalf("Expected wrapped config error, got %v", err)
	}
}

func TestLoadSiteBuildInput_UsesMarkupRenderOptions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "_posts"), 0755); err != nil {
		t.Fatalf("Failed to create posts dir: %v", err)
	}

	configContent := `title: "Markup Test"
baseURL: "https://example.com"
contentDir: "_posts"
pageDir: "pages"
staticDir: "assets"
publishDir: "public"
markup:
  allowUnsafeHTML: false
`
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	postContent := `---
title: "Unsafe Post"
date: 2026-04-27T10:00:00+08:00
---

<div class="hero">hello</div>`
	if err := os.WriteFile(filepath.Join(tmpDir, "_posts", "2026-04-27-unsafe-post.md"), []byte(postContent), 0644); err != nil {
		t.Fatalf("Failed to write post: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	input, err := loadSiteBuildInput()
	if err != nil {
		t.Fatalf("loadSiteBuildInput failed: %v", err)
	}
	if len(input.posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(input.posts))
	}
	if strings.Contains(input.posts[0].ContentHTML, `<div class="hero">`) {
		t.Fatalf("Expected markup.allowUnsafeHTML=false to disable raw HTML, got %s", input.posts[0].ContentHTML)
	}
}

func TestPrintVersion(t *testing.T) {
	var stdout bytes.Buffer
	if err := printVersion(&stdout); err != nil {
		t.Fatalf("printVersion failed: %v", err)
	}

	got := stdout.String()
	want := "Blog Static Site Generator v" + Version + "\n"
	if got != want {
		t.Fatalf("Expected %q, got %q", want, got)
	}
}

func TestPrintVersionIncludesInjectedBuildMetadata(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	oldBuildDate := BuildDate
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildDate = oldBuildDate
	})

	Version = "v9.9.9"
	Commit = "abc1234"
	BuildDate = "2026-04-27_10:20:30"

	var stdout bytes.Buffer
	if err := printVersion(&stdout); err != nil {
		t.Fatalf("printVersion failed: %v", err)
	}

	want := "Blog Static Site Generator v9.9.9\nCommit: abc1234\nBuild date: 2026-04-27_10:20:30\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Expected %q, got %q", want, got)
	}
}
