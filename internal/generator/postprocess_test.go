package generator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

func TestRewriteHTMLReferences_ReplacesKnownAssetPaths(t *testing.T) {
	html := `<!doctype html>
<html><head>
<link rel="stylesheet" href="/css/site.css">
<script src="/js/app.js"></script>
</head><body><img src="/img/cover.png"></body></html>`

	rewriteSet := map[string]string{
		"css/site.css":   "css/site.aaaaaaaaaaaa.css",
		"js/app.js":      "js/app.bbbbbbbbbbbbbbbb.js",
		"img/cover.png":  "img/cover.cccccccccccc.png",
	}

	got, found, replaced := rewriteHTMLReferences(html, buildHTMLRefPattern(keysWithLeadingSlash(rewriteSet)), rewriteSet)
	if found != 3 {
		t.Fatalf("expected 3 references found, got %d", found)
	}
	if replaced != 3 {
		t.Fatalf("expected 3 references replaced, got %d", replaced)
	}
	if !strings.Contains(got, `href="/css/site.aaaaaaaaaaaa.css"`) {
		t.Fatalf("expected CSS href to be rewritten, got %q", got)
	}
	if !strings.Contains(got, `src="/js/app.bbbbbbbbbbbbbbbb.js"`) {
		t.Fatalf("expected JS src to be rewritten, got %q", got)
	}
	if !strings.Contains(got, `src="/img/cover.cccccccccccc.png"`) {
		t.Fatalf("expected img src to be rewritten, got %q", got)
	}
}

func TestRewriteHTMLReferences_LeavesUnknownAlone(t *testing.T) {
	html := `<link rel="stylesheet" href="/css/unknown.css">`
	rewriteSet := map[string]string{
		"css/site.css": "css/site.aaaaaaaaaaaa.css",
	}
	got, found, replaced := rewriteHTMLReferences(html, buildHTMLRefPattern(keysWithLeadingSlash(rewriteSet)), rewriteSet)
	if found != 0 || replaced != 0 {
		t.Fatalf("expected no rewrites, got found=%d replaced=%d", found, replaced)
	}
	if got != html {
		t.Fatalf("expected html unchanged, got %q", got)
	}
}

func TestRewriteHTMLReferences_SkipsExternalURLs(t *testing.T) {
	html := `<a href="https://example.com/page">x</a>
<link rel="alternate" href="https://feed.example.com/rss.xml">
<script src="//cdn.example.com/lib.js"></script>`

	rewriteSet := map[string]string{
		"page":      "rewritten-page",
		"rss.xml":   "rewritten-rss",
		"lib.js":    "rewritten-lib",
	}
	got, found, replaced := rewriteHTMLReferences(html, buildHTMLRefPattern(keysWithLeadingSlash(rewriteSet)), rewriteSet)
	if found != 0 || replaced != 0 {
		t.Fatalf("external URLs should be skipped, got found=%d replaced=%d", found, replaced)
	}
	if got != html {
		t.Fatalf("expected html unchanged, got %q", got)
	}
}

func TestRewriteHTMLReferences_AttributeNameCaseInsensitive(t *testing.T) {
	html := `<link HREF="/css/site.css">`
	rewriteSet := map[string]string{
		"css/site.css": "css/site.aaaaaaaaaaaa.css",
	}
	got, _, replaced := rewriteHTMLReferences(html, buildHTMLRefPattern(keysWithLeadingSlash(rewriteSet)), rewriteSet)
	if replaced != 1 {
		t.Fatalf("expected 1 replacement, got %d", replaced)
	}
	if !strings.Contains(got, `HREF="/css/site.aaaaaaaaaaaa.css"`) {
		t.Fatalf("expected uppercase HREF to be rewritten, got %q", got)
	}
}

func TestPostprocessHTML_EndToEnd(t *testing.T) {
	outDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outDir, "css"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "css", "site.aaaaaaaaaaaa.css"), []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(`<link href="/css/site.css">`), 0644); err != nil {
		t.Fatal(err)
	}
	// also a file that should be skipped (not html)
	if err := os.WriteFile(filepath.Join(outDir, "robots.txt"), []byte("User-agent: *\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := PostprocessHTML(PostprocessOptions{
		OutputDir: outDir,
		LogicalToOutput: map[string]string{
			"css/site.css": "css/site.aaaaaaaaaaaa.css",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.HTMLFilesScanned != 1 {
		t.Fatalf("expected 1 HTML file scanned, got %d", stats.HTMLFilesScanned)
	}
	if stats.HTMLFilesChanged != 1 {
		t.Fatalf("expected 1 HTML file changed, got %d", stats.HTMLFilesChanged)
	}
	if stats.ReferencesFound != 1 || stats.ReferencesRewritten != 1 {
		t.Fatalf("expected 1/1 rewrites, got found=%d replaced=%d", stats.ReferencesFound, stats.ReferencesRewritten)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `href="/css/site.aaaaaaaaaaaa.css"`) {
		t.Fatalf("expected rewrite, got %q", string(got))
	}
}

func TestPostprocessHTML_EmptyRewriteSet(t *testing.T) {
	outDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(`<p>x</p>`), 0644); err != nil {
		t.Fatal(err)
	}
	stats, err := PostprocessHTML(PostprocessOptions{
		OutputDir:       outDir,
		LogicalToOutput: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.HTMLFilesScanned != 0 {
		t.Fatalf("expected 0 scans, got %d", stats.HTMLFilesScanned)
	}
}

func TestCollectAssetRewriteEntries(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	assets := []staticAssetFile{
		{SourcePath: mustWrite("site.css", "body{}"), OutputPath: "css/site.css"},
		{SourcePath: mustWrite("site.js", "console.log(1)"), OutputPath: "js/site.js"},
		{SourcePath: mustWrite("data.json", "{}"), OutputPath: "data/data.json"},
	}
	cfg := &config.Config{
		Assets: &config.AssetsConfig{
			Fingerprint: &config.AssetsFingerprintConfig{
				Strategy: config.AssetsFingerprintStrategyFilename,
				Extensions: []string{
					".css", ".js",
				},
			},
		},
	}
	fp := newAssetFingerprinter(cfg)
	entries, err := collectAssetRewriteEntries(assets, fp)
	if err != nil {
		t.Fatal(err)
	}
	// .json not in extensions -> logical == output, not in entries
	if len(entries) != 2 {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("expected 2 entries, got %d (%v)", len(entries), keys)
	}
	if !strings.HasPrefix(entries["css/site.css"], "css/site.") {
		t.Fatalf("expected fingerprinted css path, got %q", entries["css/site.css"])
	}
	if !strings.HasPrefix(entries["js/site.js"], "js/site.") {
		t.Fatalf("expected fingerprinted js path, got %q", entries["js/site.js"])
	}
}

func TestExtractEmbeddedHash(t *testing.T) {
	cases := map[string]struct {
		path   string
		want   string
		wantOK bool
	}{
		"css/site.aaaaaaaaaaaa.css": {"css/site.aaaaaaaaaaaa.css", "aaaaaaaaaaaa", true},
		"img/cover.cccccccccccc.png": {"img/cover.cccccccccccc.png", "cccccccccccc", true},
		"data/data.json":                {"data/data.json", "", false},
		"img/cover.tooshort.png":        {"img/cover.tooshort.png", "", false},
		"img/cover.UPPERCASE.png":       {"img/cover.UPPERCASE.png", "", false},
	}
	for path, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := extractEmbeddedHash(tc.path)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("extractEmbeddedHash(%q) = (%q, %v), want (%q, %v)", path, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestCategorizeAsset(t *testing.T) {
	cases := map[string]assetCategory{
		".css":   categoryCSS,
		".CSS":   categoryCSS,
		".js":    categoryJS,
		".mjs":   categoryJS,
		".png":   categoryImage,
		".svg":   categoryImage,
		".webp":  categoryImage,
		".json":  categoryOther,
		".woff2": categoryOther,
		"":       categoryOther,
	}
	for ext, want := range cases {
		if got := CategorizeAsset(ext); got != want {
			t.Fatalf("CategorizeAsset(%q) = %q, want %q", ext, got, want)
		}
	}
}

func TestVerifyAssetHashes_AllMatch(t *testing.T) {
	outDir := t.TempDir()
	content := []byte("body{}")
	realHash := hashBytes(content)
	hashed := "css/site." + realHash + ".css"
	absDir := filepath.Join(outDir, "css")
	if err := os.MkdirAll(absDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, hashed), content, 0644); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"assets":["` + hashed + `"]}`
	if err := os.WriteFile(filepath.Join(outDir, staticAssetManifestName), []byte(manifestJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Assets: &config.AssetsConfig{
			Fingerprint: &config.AssetsFingerprintConfig{
				Strategy:   config.AssetsFingerprintStrategyFilename,
				Extensions: []string{".css"},
			},
		},
	}
	fp := newAssetFingerprinter(cfg)

	mismatches, verified, err := VerifyAssetHashes(outDir, fp)
	if err != nil {
		t.Fatal(err)
	}
	if verified != 1 {
		t.Fatalf("expected 1 verified, got %d", verified)
	}
	if len(mismatches) != 0 {
		t.Fatalf("expected no mismatches, got %d", len(mismatches))
	}
}

func TestVerifyAssetHashes_DetectsMismatch(t *testing.T) {
	outDir := t.TempDir()
	absDir := filepath.Join(outDir, "css")
	if err := os.MkdirAll(absDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Note: the embedded hash is `aaaaaaaaaaaa` but the actual content
	// will produce a different sha256 prefix.
	css := "css/site.aaaaaaaaaaaa.css"
	if err := os.WriteFile(filepath.Join(outDir, css), []byte("body{color:red}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, staticAssetManifestName), []byte(`{"assets":["css/site.aaaaaaaaaaaa.css"]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Assets: &config.AssetsConfig{
			Fingerprint: &config.AssetsFingerprintConfig{
				Strategy:   config.AssetsFingerprintStrategyFilename,
				Extensions: []string{".css"},
			},
		},
	}
	fp := newAssetFingerprinter(cfg)

	mismatches, _, err := VerifyAssetHashes(outDir, fp)
	if err != nil {
		t.Fatal(err)
	}
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(mismatches))
	}
	if mismatches[0].OutputPath != css {
		t.Fatalf("expected mismatch on %q, got %q", css, mismatches[0].OutputPath)
	}
	if mismatches[0].ExpectedHash != "aaaaaaaaaaaa" {
		t.Fatalf("expected expected hash aaaaaaaaaaaaaaaa, got %q", mismatches[0].ExpectedHash)
	}
}

// keysWithLeadingSlash is a tiny helper to build the rewriteKeys list
// matching PostprocessHTML's internal layout.
func keysWithLeadingSlash(rewriteSet map[string]string) []string {
	keys := make([]string, 0, len(rewriteSet))
	for k := range rewriteSet {
		keys = append(keys, "/"+k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}
