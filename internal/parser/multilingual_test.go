package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePost_TranslationKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "2026-10-01-hello.md")
	content := `---
title: Hello
translationKey: hello-world
customParam: kept
---
Body text.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	post, err := ParsePost(path)
	if err != nil {
		t.Fatalf("ParsePost failed: %v", err)
	}
	if post.TranslationKey != "hello-world" {
		t.Errorf("TranslationKey = %q, want %q", post.TranslationKey, "hello-world")
	}
	// translationKey is an explicitly tagged field: it must not leak into
	// the inline Params catch-all.
	if _, ok := post.Params["translationKey"]; ok {
		t.Errorf("translationKey leaked into Params: %v", post.Params)
	}
	if post.Params["customParam"] != "kept" {
		t.Errorf("customParam missing from Params: %v", post.Params)
	}
}

func TestParsePosts_ExcludeLanguageSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	postsDir := filepath.Join(tmpDir, "_posts")
	zhDir := filepath.Join(postsDir, "zh")
	if err := os.MkdirAll(zhDir, 0755); err != nil {
		t.Fatal(err)
	}

	writePost := func(dir, name, title string) {
		content := "---\ntitle: " + title + "\n---\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writePost(postsDir, "2026-10-01-en-post.md", "EN Post")
	writePost(zhDir, "2026-10-01-zh-post.md", "ZH Post")

	// Without exclusion the recursive walk picks up the nested language
	// directory (this is the leak the exclusion exists to prevent).
	all, err := ParsePosts(postsDir)
	if err != nil {
		t.Fatalf("ParsePosts failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ParsePosts without exclusion returned %d posts, want 2", len(all))
	}

	filtered, err := ParsePostsWithOptionsConcurrentExclude(postsDir, DefaultRenderOptions(), 1, []string{zhDir})
	if err != nil {
		t.Fatalf("ParsePostsWithOptionsConcurrentExclude failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("excluded parse returned %d posts, want 1", len(filtered))
	}
	if filtered[0].Title != "EN Post" {
		t.Errorf("excluded parse kept %q, want %q", filtered[0].Title, "EN Post")
	}

	// The language directory itself parses independently.
	zhPosts, err := ParsePosts(zhDir)
	if err != nil {
		t.Fatalf("ParsePosts(zh) failed: %v", err)
	}
	if len(zhPosts) != 1 || zhPosts[0].Title != "ZH Post" {
		t.Errorf("language dir parse = %v posts, want 1 ZH Post", len(zhPosts))
	}
}

func TestParsePages_ExcludeLanguageSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	pagesDir := filepath.Join(tmpDir, "pages")
	zhDir := filepath.Join(pagesDir, "zh")
	if err := os.MkdirAll(zhDir, 0755); err != nil {
		t.Fatal(err)
	}

	writePage := func(dir, name, title string) {
		content := "---\ntitle: " + title + "\n---\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writePage(pagesDir, "about.md", "About")
	writePage(zhDir, "about.md", "关于")

	filtered, err := ParsePagesWithOptionsConcurrentExclude(pagesDir, DefaultRenderOptions(), 1, []string{zhDir})
	if err != nil {
		t.Fatalf("ParsePagesWithOptionsConcurrentExclude failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "About" {
		t.Fatalf("excluded parse = %v pages, want 1 About page", len(filtered))
	}
}

func TestParsePosts_ExcludeMixedSpelling(t *testing.T) {
	// Exclusion must hold even when the walked dir and the exclude dir
	// are spelled differently: a "./"-prefixed walk root, or a relative
	// walk root with an absolute exclude dir. Both used to silently
	// leak the language subdirectory into the default parse.
	tmpDir := t.TempDir()
	postsDir := filepath.Join(tmpDir, "_posts")
	zhDir := filepath.Join(postsDir, "zh")
	if err := os.MkdirAll(zhDir, 0755); err != nil {
		t.Fatal(err)
	}
	writePost := func(dir, name, title string) {
		content := "---\ntitle: " + title + "\n---\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writePost(postsDir, "2026-10-01-en-post.md", "EN Post")
	writePost(zhDir, "2026-10-01-zh-post.md", "ZH Post")

	// Work inside tmpDir so relative spellings resolve there. Resolve
	// symlinks first: filepath.Abs is lexical against os.Getwd, which
	// returns the resolved path (macOS /var -> /private/var), while the
	// absolute spellings below must match it exactly.
	resolvedDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	postsDir = filepath.Join(resolvedDir, "_posts")
	zhDir = filepath.Join(postsDir, "zh")
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(resolvedDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	cases := []struct {
		name    string
		dir     string
		exclude string
	}{
		{"dot-slash walk root", "./_posts", "_posts/zh"},
		{"absolute exclude over relative walk", "_posts", zhDir},
		{"relative exclude over absolute walk", postsDir, "_posts/zh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filtered, err := ParsePostsWithOptionsConcurrentExclude(tc.dir, DefaultRenderOptions(), 1, []string{tc.exclude})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if len(filtered) != 1 || filtered[0].Title != "EN Post" {
				t.Errorf("dir=%q exclude=%q: got %d posts, want only EN Post", tc.dir, tc.exclude, len(filtered))
			}
		})
	}
}
