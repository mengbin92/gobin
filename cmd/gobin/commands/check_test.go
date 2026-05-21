package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheck_ReportsSuccessForScaffoldedSite(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")

	if err := initializeSite(io.Discard, siteDir); err != nil {
		t.Fatalf("initializeSite failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	if err := runCheck(&stdout, &stderr, false); err != nil {
		t.Fatalf("runCheck failed: %v\nstderr=%s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "config loaded") || !strings.Contains(out, "templates loaded") || !strings.Contains(out, "Site check passed.") {
		t.Fatalf("Expected check OK lines, got %q", out)
	}
}

func TestRunCheck_ReportsConfigErrors(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	err := runCheck(&stdout, &stderr, false)
	if err == nil {
		t.Fatal("Expected check to fail without config")
	}
	if !strings.Contains(stderr.String(), "[FAIL] config") {
		t.Fatalf("Expected config FAIL message, got %q", stderr.String())
	}
}

func TestRunCheck_DetectsPermalinkCollision(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `title: "Collision Test"
baseURL: "https://example.com"
contentDir: "_posts"
pageDir: "pages"
staticDir: "assets"
publishDir: "public"
permalinks:
  posts: "/posts/:slug/"
`
	mustWriteFile(t, filepath.Join(tmpDir, "config.yaml"), configContent)

	// Two posts that resolve to the same slug ("hello-world") and therefore
	// the same permalink under permalinks.posts = "/posts/:slug/".
	postA := `---
title: "Hello World"
slug: "hello-world"
date: 2026-05-21T10:00:00+08:00
---

A`
	postB := `---
title: "Hello, World!"
slug: "hello-world"
date: 2026-05-21T11:00:00+08:00
---

B`
	mustWriteFile(t, filepath.Join(tmpDir, "_posts", "2026-05-21-hello-a.md"), postA)
	mustWriteFile(t, filepath.Join(tmpDir, "_posts", "2026-05-21-hello-b.md"), postB)

	// Add a minimal template so DryRun does not fail on template loading.
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ template "main" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "main" }}list{{ end }}{{ define "listMain" }}list{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "main" }}single{{ end }}{{ define "singleMain" }}single{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	var stdout, stderr bytes.Buffer
	err := runCheck(&stdout, &stderr, false)
	if err == nil {
		t.Fatal("Expected colliding permalinks to fail check")
	}
	if !strings.Contains(stderr.String(), "permalink collision") {
		t.Fatalf("Expected permalink collision FAIL line, got stderr=%q", stderr.String())
	}
}
