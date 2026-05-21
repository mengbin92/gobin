package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func TestReadWriteBuildManifest_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := &BuildManifest{
		Version:      buildManifestVersion,
		BuildEnvHash: "envhash",
		Posts: []BuildManifestPostEntry{
			{SourcePath: "_posts/2026-01-01-hello.md", SourceHash: "hash1", OutputPath: "2026/01/01/hello/index.html", ListHash: "L1", FeedHash: "F1", SearchHash: "S1", SitemapHash: "M1"},
		},
		Pages: []BuildManifestPageEntry{
			{SourcePath: "pages/about.md", SourceHash: "hash2", OutputPath: "about/index.html"},
		},
	}

	if err := writeBuildManifest(tmpDir, manifest); err != nil {
		t.Fatalf("writeBuildManifest failed: %v", err)
	}

	loaded, err := readBuildManifest(tmpDir)
	if err != nil {
		t.Fatalf("readBuildManifest failed: %v", err)
	}
	if loaded.BuildEnvHash != "envhash" {
		t.Fatalf("expected env hash to round-trip, got %q", loaded.BuildEnvHash)
	}
	if len(loaded.Posts) != 1 || loaded.Posts[0].SourcePath != "_posts/2026-01-01-hello.md" {
		t.Fatalf("post round-trip mismatch: %#v", loaded.Posts)
	}
	if len(loaded.Pages) != 1 || loaded.Pages[0].SourcePath != "pages/about.md" {
		t.Fatalf("page round-trip mismatch: %#v", loaded.Pages)
	}
}

func TestReadBuildManifest_MissingFileReturnsEmpty(t *testing.T) {
	manifest, err := readBuildManifest(t.TempDir())
	if err != nil {
		t.Fatalf("readBuildManifest returned error on missing file: %v", err)
	}
	if manifest.Version != buildManifestVersion {
		t.Fatalf("expected current version on empty manifest, got %d", manifest.Version)
	}
	if len(manifest.Posts) != 0 || len(manifest.Pages) != 0 {
		t.Fatal("expected empty entries on missing manifest")
	}
}

func TestReadBuildManifest_CorruptFallsBackToEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, buildManifestName), []byte("not json"), 0644); err != nil {
		t.Fatalf("write corrupt manifest: %v", err)
	}
	manifest, err := readBuildManifest(tmpDir)
	if err != nil {
		t.Fatalf("readBuildManifest returned error on corrupt file: %v", err)
	}
	if manifest.Version != buildManifestVersion {
		t.Fatalf("expected current version on corrupt manifest, got %d", manifest.Version)
	}
}

func TestReadBuildManifest_VersionMismatchFallsBackToEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	stale := []byte(`{"version":99,"build_env_hash":"x","posts":[{"source_path":"old","source_hash":"x"}],"pages":[]}`)
	if err := os.WriteFile(filepath.Join(tmpDir, buildManifestName), stale, 0644); err != nil {
		t.Fatalf("write stale manifest: %v", err)
	}
	manifest, err := readBuildManifest(tmpDir)
	if err != nil {
		t.Fatalf("readBuildManifest returned error on stale version: %v", err)
	}
	if len(manifest.Posts) != 0 {
		t.Fatal("expected stale-version manifest to be discarded")
	}
}

func TestWriteBuildManifest_StableOrdering(t *testing.T) {
	tmpDir := t.TempDir()

	unordered := &BuildManifest{
		Version:      buildManifestVersion,
		BuildEnvHash: "envhash",
		Posts: []BuildManifestPostEntry{
			{SourcePath: "_posts/b.md", SourceHash: "h2"},
			{SourcePath: "_posts/a.md", SourceHash: "h1"},
		},
	}
	if err := writeBuildManifest(tmpDir, unordered); err != nil {
		t.Fatalf("writeBuildManifest failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(tmpDir, buildManifestName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(raw), `"_posts/a.md"`) || !strings.Contains(string(raw), `"_posts/b.md"`) {
		t.Fatalf("unexpected manifest body: %s", raw)
	}
	indexA := strings.Index(string(raw), `"_posts/a.md"`)
	indexB := strings.Index(string(raw), `"_posts/b.md"`)
	if indexA > indexB {
		t.Fatalf("expected entries to be sorted by source path, got:\n%s", raw)
	}
}

func TestBuildManifestForRun_HashesSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	postPath := filepath.Join(tmpDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")
	pagePath := filepath.Join(tmpDir, "pages", "about.md")
	mustWriteFile(t, pagePath, "page body")

	posts := []*parser.Post{{
		Title:    "Hello",
		Slug:     "hello",
		URL:      "/hello/",
		FilePath: postPath,
	}}
	pages := []*parser.Page{{
		Title:    "About",
		Slug:     "about",
		URL:      "/about/",
		FilePath: pagePath,
	}}

	cfg := config.Normalize(&config.Config{})
	manifest, err := buildManifestForRun(cfg, posts, pages)
	if err != nil {
		t.Fatalf("buildManifestForRun failed: %v", err)
	}
	if manifest.BuildEnvHash == "" {
		t.Fatal("expected non-empty build env hash")
	}
	if len(manifest.Posts) != 1 || manifest.Posts[0].SourceHash != hashBytes([]byte("post body")) {
		t.Fatalf("post entry mismatch: %#v", manifest.Posts)
	}
	if manifest.Posts[0].OutputPath != "hello/index.html" {
		t.Fatalf("expected post output path, got %q", manifest.Posts[0].OutputPath)
	}
	if len(manifest.Pages) != 1 || manifest.Pages[0].SourceHash != hashBytes([]byte("page body")) {
		t.Fatalf("page entry mismatch: %#v", manifest.Pages)
	}
}

func TestBuildManifestForRun_SkipsEntriesWithoutSourcePath(t *testing.T) {
	cfg := config.Normalize(&config.Config{})
	manifest, err := buildManifestForRun(cfg, []*parser.Post{{Title: "no source"}}, []*parser.Page{{Title: "no source"}})
	if err != nil {
		t.Fatalf("buildManifestForRun failed: %v", err)
	}
	if len(manifest.Posts) != 0 || len(manifest.Pages) != 0 {
		t.Fatal("expected entries without FilePath to be skipped")
	}
}

func TestComputeBuildEnvHash_ChangesWithConfig(t *testing.T) {
	cfgA := config.Normalize(&config.Config{Title: "A"})
	cfgB := config.Normalize(&config.Config{Title: "B"})

	opts := parser.DefaultRenderOptions()
	hashA, err := computeBuildEnvHash(cfgA, opts)
	if err != nil {
		t.Fatalf("hashA failed: %v", err)
	}
	hashB, err := computeBuildEnvHash(cfgB, opts)
	if err != nil {
		t.Fatalf("hashB failed: %v", err)
	}
	if hashA == hashB {
		t.Fatal("expected different config to yield different build env hash")
	}
}

func TestGenerate_WritesBuildManifest(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")

	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ template "main" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "list.html"), `{{ define "main" }}list{{ end }}{{ define "listMain" }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), `{{ define "main" }}single{{ end }}{{ define "singleMain" }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	post := &parser.Post{
		Title:    "Hello",
		Slug:     "hello",
		URL:      "/hello/",
		FilePath: postPath,
	}

	cfg := &config.Config{
		Title:        "Manifest Test",
		BaseURL:      "https://example.com",
		StaticDir:    "assets",
		ThemesDir:    "themes",
		Paginate:     1,
		PaginatePath: "page",
	}

	if err := Generate([]*parser.Post{post}, cfg, outputDir, false, false, true); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	manifestPath := filepath.Join(outputDir, buildManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("expected manifest at %s, got err=%v", manifestPath, err)
	}

	var got BuildManifest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if got.Version != buildManifestVersion {
		t.Fatalf("expected manifest version %d, got %d", buildManifestVersion, got.Version)
	}
	if got.BuildEnvHash == "" {
		t.Fatal("expected non-empty build env hash in written manifest")
	}
	if len(got.Posts) != 1 {
		t.Fatalf("expected one post entry, got %d", len(got.Posts))
	}
	if got.Posts[0].OutputPath != "hello/index.html" {
		t.Fatalf("expected post output path hello/index.html, got %q", got.Posts[0].OutputPath)
	}
	if got.Posts[0].SourceHash == "" {
		t.Fatal("expected non-empty source hash")
	}
}
