package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

func TestCopyStaticAssetsWithResult_RemovesPreviouslyManagedStaleAssets(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	outputDir := filepath.Join(tmpDir, "public")
	sourcePath := filepath.Join(staticDir, "css", "main.css")

	mustWriteFile(t, sourcePath, "body { color: black; }")

	cfg := &config.Config{StaticDir: staticDir}
	if _, err := copyStaticAssetsWithResult(cfg, outputDir); err != nil {
		t.Fatalf("initial copyStaticAssetsWithResult failed: %v", err)
	}

	staleOutputPath := filepath.Join(outputDir, "css", "main.css")
	if _, err := os.Stat(staleOutputPath); err != nil {
		t.Fatalf("expected copied asset to exist before removal: %v", err)
	}

	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("failed to remove source asset: %v", err)
	}
	result, err := copyStaticAssetsWithResult(cfg, outputDir)
	if err != nil {
		t.Fatalf("second copyStaticAssetsWithResult failed: %v", err)
	}

	if result.Deleted != 1 {
		t.Fatalf("expected one stale managed asset to be deleted, got %d", result.Deleted)
	}
	if _, err := os.Stat(staleOutputPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale managed asset to be removed, got err=%v", err)
	}
}

func TestCopyStaticAssetsWithResult_DoesNotRemoveUnmanagedOutputFiles(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	outputDir := filepath.Join(tmpDir, "public")

	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")
	if _, err := copyStaticAssetsWithResult(&config.Config{StaticDir: staticDir}, outputDir); err != nil {
		t.Fatalf("copyStaticAssetsWithResult failed: %v", err)
	}

	unmanagedPath := filepath.Join(outputDir, "manual.txt")
	mustWriteFile(t, unmanagedPath, "keep")

	if _, err := copyStaticAssetsWithResult(&config.Config{StaticDir: staticDir}, outputDir); err != nil {
		t.Fatalf("second copyStaticAssetsWithResult failed: %v", err)
	}

	if content, err := os.ReadFile(unmanagedPath); err != nil || string(content) != "keep" {
		t.Fatalf("expected unmanaged output file to remain, content=%q err=%v", string(content), err)
	}
}

func TestPlanStaticAssetCopies_RejectsOutputPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "assets", "main.css")
	mustWriteFile(t, sourcePath, "body {}")

	_, err := planStaticAssetCopiesForFiles([]staticAssetFile{
		{
			SourcePath: sourcePath,
			OutputPath: filepath.Join("..", "escaped.css"),
		},
	}, filepath.Join(tmpDir, "public"), newAssetFingerprinter(nil))
	if err == nil {
		t.Fatal("expected asset output path traversal to fail")
	}
	if !strings.Contains(err.Error(), "escapes output directory") {
		t.Fatalf("expected output directory escape error, got %v", err)
	}
}

func TestAssetURLResolver_AppendsContentVersion(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")

	resolver, err := newAssetURLResolver(&config.Config{StaticDir: staticDir})
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/css/main.css?theme=light#top")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	wantHash := shortSHA256("body {}")
	if got != "/css/main.css?theme=light&v="+wantHash+"#top" {
		t.Fatalf("expected versioned asset URL, got %q", got)
	}
}

func TestAssetURLResolver_PrefixesBaseURLPath(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")

	resolver, err := newAssetURLResolver(&config.Config{
		BaseURL:   "https://example.com/blog/",
		StaticDir: staticDir,
	})
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/css/main.css")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	want := "/blog/css/main.css?v=" + shortSHA256("body {}")
	if got != want {
		t.Fatalf("expected base path prefixed asset URL %q, got %q", want, got)
	}
}

func TestAssetURLResolver_LeavesExternalAndUnknownURLsUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")

	resolver, err := newAssetURLResolver(&config.Config{StaticDir: staticDir})
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	for _, raw := range []string{"https://cdn.example.com/app.css", "//cdn.example.com/app.css", "/css/missing.css"} {
		got, err := resolver.URL(raw)
		if err != nil {
			t.Fatalf("asset URL %q failed: %v", raw, err)
		}
		if got != raw {
			t.Fatalf("expected %q to remain unchanged, got %q", raw, got)
		}
	}
}

func TestAssetURLResolver_UsesOverlayWinner(t *testing.T) {
	tmpDir := t.TempDir()
	mustWriteFile(t, filepath.Join(tmpDir, "themes", "demo", "assets", "css", "theme.css"), "theme")
	mustWriteFile(t, filepath.Join(tmpDir, "assets", "css", "theme.css"), "site")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	resolver, err := newAssetURLResolver(&config.Config{
		StaticDir: "assets",
		ThemesDir: "themes",
		Theme:     "demo",
	})
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/css/theme.css")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	if got != "/css/theme.css?v="+shortSHA256("site") {
		t.Fatalf("expected site asset version to win, got %q", got)
	}
}

func TestSiteURLPath_PrefixesBaseURLPath(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{name: "root base", baseURL: "https://example.com", path: "/docs/", want: "/docs/"},
		{name: "path base", baseURL: "https://example.com/gobin/", path: "/docs/", want: "/gobin/docs/"},
		{name: "fragment", baseURL: "https://example.com/gobin/", path: "#quickstart", want: "#quickstart"},
		{name: "external", baseURL: "https://example.com/gobin/", path: "https://github.com/mengbin92/gobin", want: "https://github.com/mengbin92/gobin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := siteURLPath(tt.baseURL, tt.path); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func shortSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:12]
}

func filenameFingerprintConfig(staticDir string) *config.Config {
	return &config.Config{
		StaticDir: staticDir,
		Assets: &config.AssetsConfig{
			Fingerprint: &config.AssetsFingerprintConfig{
				Strategy:   config.AssetsFingerprintStrategyFilename,
				Extensions: []string{".css", ".js"},
			},
		},
	}
}

func TestCopyStaticAssetsWithResult_FilenameFingerprint_WritesHashedName(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	outputDir := filepath.Join(tmpDir, "public")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")
	mustWriteFile(t, filepath.Join(staticDir, "robots.txt"), "User-agent: *")

	cfg := filenameFingerprintConfig(staticDir)

	result, err := copyStaticAssetsWithResult(cfg, outputDir)
	if err != nil {
		t.Fatalf("copyStaticAssetsWithResult failed: %v", err)
	}
	if result.Copied != 2 {
		t.Fatalf("expected 2 copied assets, got %d", result.Copied)
	}

	wantHash := shortSHA256("body {}")
	hashedPath := filepath.Join(outputDir, "css", "main."+wantHash+".css")
	if _, err := os.Stat(hashedPath); err != nil {
		t.Fatalf("expected fingerprinted asset at %s, got err=%v", hashedPath, err)
	}
	originalPath := filepath.Join(outputDir, "css", "main.css")
	if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
		t.Fatalf("expected original filename to be absent, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "robots.txt")); err != nil {
		t.Fatalf("expected non-fingerprintable asset to land at original path, got err=%v", err)
	}
}

func TestCopyStaticAssetsWithResult_FilenameFingerprint_RemovesStaleHashWhenContentChanges(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	outputDir := filepath.Join(tmpDir, "public")
	sourcePath := filepath.Join(staticDir, "css", "main.css")
	mustWriteFile(t, sourcePath, "body {}")

	cfg := filenameFingerprintConfig(staticDir)
	if _, err := copyStaticAssetsWithResult(cfg, outputDir); err != nil {
		t.Fatalf("first copy failed: %v", err)
	}

	originalHash := shortSHA256("body {}")
	originalHashedPath := filepath.Join(outputDir, "css", "main."+originalHash+".css")
	if _, err := os.Stat(originalHashedPath); err != nil {
		t.Fatalf("expected original hashed asset to exist before update, got err=%v", err)
	}

	mustWriteFile(t, sourcePath, "body { color: red; }")
	result, err := copyStaticAssetsWithResult(cfg, outputDir)
	if err != nil {
		t.Fatalf("second copy failed: %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("expected one stale fingerprinted file to be deleted, got %d", result.Deleted)
	}
	if _, err := os.Stat(originalHashedPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale fingerprinted file to be removed, got err=%v", err)
	}

	updatedHash := shortSHA256("body { color: red; }")
	updatedHashedPath := filepath.Join(outputDir, "css", "main."+updatedHash+".css")
	if _, err := os.Stat(updatedHashedPath); err != nil {
		t.Fatalf("expected new fingerprinted asset at %s, got err=%v", updatedHashedPath, err)
	}
}

func TestAssetURLResolver_FilenameStrategy_ReturnsHashedPath(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")

	resolver, err := newAssetURLResolver(filenameFingerprintConfig(staticDir))
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/css/main.css")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	want := "/css/main." + shortSHA256("body {}") + ".css"
	if got != want {
		t.Fatalf("expected fingerprinted asset URL %q, got %q", want, got)
	}
}

func TestAssetURLResolver_FilenameStrategy_NonFingerprintableFallsBackToQuery(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "robots.txt"), "User-agent: *")

	resolver, err := newAssetURLResolver(filenameFingerprintConfig(staticDir))
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/robots.txt")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	if got != "/robots.txt?v="+shortSHA256("User-agent: *") {
		t.Fatalf("expected non-fingerprintable assets to keep query versioning, got %q", got)
	}
}

func TestAssetURLResolver_FilenameStrategy_PrefixesBaseURLPath(t *testing.T) {
	tmpDir := t.TempDir()
	staticDir := filepath.Join(tmpDir, "assets")
	mustWriteFile(t, filepath.Join(staticDir, "css", "main.css"), "body {}")

	cfg := filenameFingerprintConfig(staticDir)
	cfg.BaseURL = "https://example.com/blog/"
	resolver, err := newAssetURLResolver(cfg)
	if err != nil {
		t.Fatalf("newAssetURLResolver failed: %v", err)
	}

	got, err := resolver.URL("/css/main.css")
	if err != nil {
		t.Fatalf("asset URL failed: %v", err)
	}
	want := "/blog/css/main." + shortSHA256("body {}") + ".css"
	if got != want {
		t.Fatalf("expected base-path-prefixed fingerprinted URL %q, got %q", want, got)
	}
}

func TestCollectStaticAssetFiles_MultipleStaticDirs(t *testing.T) {
	tmpDir := t.TempDir()
	assetsDir := filepath.Join(tmpDir, "assets")
	imgDir := filepath.Join(tmpDir, "img")
	imagesDir := filepath.Join(tmpDir, "images")

	mustWriteFile(t, filepath.Join(assetsDir, "css", "main.css"), "body {}")
	mustWriteFile(t, filepath.Join(imgDir, "cover.jpg"), "img")
	mustWriteFile(t, filepath.Join(imagesDir, "banner.png"), "images")

	cfg := &config.Config{
		StaticDir:  "assets",
		StaticDirs: []string{assetsDir, imgDir, imagesDir},
	}
	assets, err := collectStaticAssetFiles(cfg)
	if err != nil {
		t.Fatalf("collectStaticAssetFiles failed: %v", err)
	}

	got := map[string]string{}
	for _, a := range assets {
		got[a.OutputPath] = a.SourcePath
	}
	for _, want := range []string{
		filepath.Join("css", "main.css"),
		filepath.Join("img", "cover.jpg"),
		filepath.Join("images", "banner.png"),
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected asset %q to be collected, got %#v", want, got)
		}
	}
	// Additional static dirs keep their directory name as the output prefix.
	if got[filepath.Join("img", "cover.jpg")] != filepath.Join(imgDir, "cover.jpg") {
		t.Errorf("img asset source mismatch: %#v", got)
	}
	if got[filepath.Join("images", "banner.png")] != filepath.Join(imagesDir, "banner.png") {
		t.Errorf("images asset source mismatch: %#v", got)
	}
}

func TestCopyStaticAssets_MultipleStaticDirs(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "public")
	assetsDir := filepath.Join(tmpDir, "assets")
	imgDir := filepath.Join(tmpDir, "img")

	mustWriteFile(t, filepath.Join(assetsDir, "css", "main.css"), "body {}")
	mustWriteFile(t, filepath.Join(imgDir, "cover.jpg"), "img")

	cfg := &config.Config{
		StaticDir:  "assets",
		StaticDirs: []string{assetsDir, imgDir},
	}
	if _, err := copyStaticAssetsWithResult(cfg, outputDir); err != nil {
		t.Fatalf("copyStaticAssetsWithResult failed: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("css", "main.css"),
		filepath.Join("img", "cover.jpg"),
	} {
		if _, err := os.Stat(filepath.Join(outputDir, rel)); err != nil {
			t.Errorf("expected copied asset %q, got err=%v", rel, err)
		}
	}
}
