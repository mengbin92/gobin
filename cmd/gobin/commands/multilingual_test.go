package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMultilingualCLISte creates a minimal bilingual site (en default +
// zh under _posts/zh/) using the repo template set, and chdirs into it.
func setupMultilingualCLISite(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")

	if err := initializeSite(io.Discard, siteDir); err != nil {
		t.Fatalf("initializeSite failed: %v", err)
	}

	configYAML := `title: Bilingual Blog
description: EN description
languageCode: en
baseURL: https://example.com
contentDir: _posts
publishDir: public
paginate: 10
enableRobotsTXT: true
strings:
  readMore: Read more
languages:
  zh:
    languageCode: zh-CN
    title: 双语博客
    strings:
      readMore: 阅读更多
`
	if err := os.WriteFile(filepath.Join(siteDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	zhDir := filepath.Join(siteDir, "_posts", "zh")
	if err := os.MkdirAll(zhDir, 0755); err != nil {
		t.Fatal(err)
	}
	zhPost := `---
title: 你好世界
slug: hello-zh
date: 2026-03-20
translationKey: hello
---
中文正文。
`
	if err := os.WriteFile(filepath.Join(zhDir, "2026-03-20-hello.md"), []byte(zhPost), 0644); err != nil {
		t.Fatal(err)
	}

	// Give the default-language welcome post a matching translationKey.
	enPost := `---
title: Hello World
slug: hello
date: 2026-03-20
translationKey: hello
---
English body.
`
	if err := os.WriteFile(filepath.Join(siteDir, "_posts", "2026-03-20-hello.md"), []byte(enPost), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })
	return siteDir
}

func TestRunBuild_Multilingual(t *testing.T) {
	siteDir := setupMultilingualCLISite(t)

	var stdout bytes.Buffer
	if err := runBuild(&stdout, false, false, true, false, 0); err != nil {
		t.Fatalf("runBuild failed: %v", err)
	}

	expectedPaths := []string{
		"public/index.html",
		"public/hello/index.html",
		"public/robots.txt",
		"public/.gobin-build.json",
		"public/zh/index.html",
		"public/zh/hello-zh/index.html",
		"public/zh/index.xml",
		"public/zh/sitemap.xml",
		"public/zh/search-index.json",
		"public/zh/404.html",
		"public/zh/.gobin-build.json",
	}
	for _, rel := range expectedPaths {
		if _, err := os.Stat(filepath.Join(siteDir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(siteDir, "public", "zh", "robots.txt")); !os.IsNotExist(err) {
		t.Error("zh/robots.txt should not exist")
	}

	// The zh page renders in its own language and links back to the en
	// translation.
	zhPage, err := os.ReadFile(filepath.Join(siteDir, "public", "zh", "hello-zh", "index.html"))
	if err != nil {
		t.Fatalf("read zh page: %v", err)
	}
	if !strings.Contains(string(zhPage), `<html lang="zh-CN"`) {
		t.Error("zh page missing lang attribute")
	}
	if !strings.Contains(string(zhPage), "https://example.com/zh/hello-zh/") {
		t.Error("zh page canonical missing language prefix")
	}

	// The default-language page must not contain zh content.
	enIndex, err := os.ReadFile(filepath.Join(siteDir, "public", "index.html"))
	if err != nil {
		t.Fatalf("read en index: %v", err)
	}
	if strings.Contains(string(enIndex), "你好世界") {
		t.Error("zh post leaked into the default-language index")
	}
}

func TestRunCheck_Multilingual(t *testing.T) {
	setupMultilingualCLISite(t)

	var stdout, stderr bytes.Buffer
	if err := runCheck(&stdout, &stderr, false, false); err != nil {
		t.Fatalf("runCheck failed: %v\nstderr: %s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "language zh: 1 post(s)") {
		t.Errorf("check output should report per-language counts, got %q", output)
	}
	if !strings.Contains(output, "Site check passed.") {
		t.Errorf("check should pass, got %q", output)
	}
}

func TestRunCheck_MultilingualCollision(t *testing.T) {
	siteDir := setupMultilingualCLISite(t)

	// Two zh posts with the same slug collide inside the zh namespace.
	dup := `---
title: Duplicate
slug: hello-zh
date: 2026-03-21
---
dup body.
`
	if err := os.WriteFile(filepath.Join(siteDir, "_posts", "zh", "2026-03-21-dup.md"), []byte(dup), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck(&stdout, &stderr, false, false)
	if err == nil {
		t.Fatal("runCheck should fail on a within-language collision")
	}
	if !strings.Contains(stderr.String(), "zh/hello-zh/index.html") {
		t.Errorf("collision should be reported with the zh/ prefix, got %q", stderr.String())
	}
}
