package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func multilingualServeConfig() *config.Config {
	return config.Normalize(&config.Config{
		Title: "ML",
		Languages: map[string]*config.LanguageConfig{
			"zh": {}, // nested convention dirs: _posts/zh, pages/zh
			"ja": {ContentDir: "content/ja", PageDir: "content/ja-pages"},
		},
	})
}

func TestWatchPaths_IncludesLanguageDirs(t *testing.T) {
	cfg := multilingualServeConfig()
	paths := watchPaths(cfg)

	want := []string{"_posts/zh", "pages/zh", "content/ja", "content/ja-pages"}
	for _, dir := range want {
		found := false
		for _, p := range paths {
			if filepath.Clean(p) == filepath.Clean(dir) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("watchPaths missing %s (got %v)", dir, paths)
		}
	}
}

func TestClassifyChange_LanguageDirs(t *testing.T) {
	cfg := multilingualServeConfig()

	tests := []struct {
		path string
		want changeKind
	}{
		{filepath.Join("_posts", "zh", "2026-01-01-a.md"), changeContent},
		{filepath.Join("pages", "zh", "about.md"), changePage},
		{filepath.Join("content", "ja", "2026-01-01-a.md"), changeContent},
		{filepath.Join("content", "ja-pages", "about.md"), changePage},
		{filepath.Join("_posts", "2026-01-01-a.md"), changeContent},
		{filepath.Join("content", "ja", "image.png"), changeStructural},
	}
	for _, tt := range tests {
		if got := classifyChange(tt.path, cfg); got != tt.want {
			t.Errorf("classifyChange(%s) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestContentCache_MultilingualAssemble(t *testing.T) {
	cfg := multilingualServeConfig()
	input := &siteBuildInput{
		cfg:   cfg,
		posts: []*parser.Post{{Title: "EN", FilePath: filepath.Join("_posts", "2026-01-01-en.md")}},
		langPosts: map[string][]*parser.Post{
			"zh": {{Title: "中文", FilePath: filepath.Join("_posts", "zh", "2026-01-01-zh.md")}},
			"ja": {{Title: "日本語", FilePath: filepath.Join("content", "ja", "2026-01-01-ja.md")}},
		},
		langPages: map[string][]*parser.Page{
			"zh": {{Title: "关于", FilePath: filepath.Join("pages", "zh", "about.md")}},
		},
	}

	cache := &contentCache{}
	if err := cache.refreshAll(input); err != nil {
		t.Fatalf("refreshAll failed: %v", err)
	}

	assembled := cache.assemble()
	if len(assembled.posts) != 1 || assembled.posts[0].Title != "EN" {
		t.Errorf("default posts = %v, want 1 EN post", assembled.posts)
	}
	if len(assembled.langPosts["zh"]) != 1 || assembled.langPosts["zh"][0].Title != "中文" {
		t.Errorf("zh posts = %v, want 1 中文 post", assembled.langPosts["zh"])
	}
	if len(assembled.langPosts["ja"]) != 1 || assembled.langPosts["ja"][0].Title != "日本語" {
		t.Errorf("ja posts = %v, want 1 日本語 post", assembled.langPosts["ja"])
	}
	if len(assembled.langPages["zh"]) != 1 || assembled.langPages["zh"][0].Title != "关于" {
		t.Errorf("zh pages = %v, want 1 关于 page", assembled.langPages["zh"])
	}
}

func TestContentCache_IncrementalReparseLanguageFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	config.Normalize(&config.Config{
		Title: "ML",
		Languages: map[string]*config.LanguageConfig{
			"zh": {},
		},
	})

	configYAML := `title: ML
languages:
  zh: {}
`
	if err := os.WriteFile("config.yaml", []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	zhPostPath := filepath.Join("_posts", "zh", "2026-01-01-zh.md")
	if err := os.MkdirAll(filepath.Dir(zhPostPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zhPostPath, []byte("---\ntitle: 旧标题\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	zhPagePath := filepath.Join("pages", "zh", "about.md")
	if err := os.MkdirAll(filepath.Dir(zhPagePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zhPagePath, []byte("---\ntitle: 关于\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	input, err := loadSiteBuildInput()
	if err != nil {
		t.Fatalf("loadSiteBuildInput failed: %v", err)
	}
	cache := &contentCache{}
	changes := &changeSet{}
	if err := cache.refreshAll(input); err != nil {
		t.Fatalf("refreshAll failed: %v", err)
	}

	// Edit the zh post, then run one incremental load cycle.
	if err := os.WriteFile(zhPostPath, []byte("---\ntitle: 新标题\n---\nbody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	changes.add(zhPostPath, changeContent)
	changes.add(zhPagePath, changePage)

	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)
	updated, err := loader()
	if err != nil {
		t.Fatalf("incremental load failed: %v", err)
	}

	if len(updated.langPosts["zh"]) != 1 || updated.langPosts["zh"][0].Title != "新标题" {
		t.Errorf("zh posts after reparse = %v, want 1 post titled 新标题", updated.langPosts["zh"])
	}
	// The language page must be re-parsed against its own pageDir so its
	// URL stays /about/ (the /<lang>/ prefix comes from the output dir).
	if len(updated.langPages["zh"]) != 1 || updated.langPages["zh"][0].URL != "/about/" {
		t.Errorf("zh page URL = %v, want /about/", updated.langPages["zh"])
	}
}
