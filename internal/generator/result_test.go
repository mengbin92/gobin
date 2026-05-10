package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

func TestGenerateWithPagesResultReportsStaticAssetStats(t *testing.T) {
	tmpDir := t.TempDir()
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

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "list.html"), `{{ define "listPage" }}list{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "404.html"), `{{ define "notFoundPage" }}404{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsPage" }}terms{{ end }}{{ define "taxonomyPage" }}taxonomy{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "assets", "css", "main.css"), `body{}`)

	cfg := &config.Config{
		BaseURL:    "/",
		StaticDir:  "assets",
		PublishDir: "public",
		Outputs: &config.OutputsConfig{
			Feed:    boolPtr(false),
			Search:  boolPtr(false),
			Sitemap: boolPtr(false),
			Robots:  boolPtr(false),
		},
	}

	first, err := GenerateWithPagesResult(nil, nil, cfg, cfg.PublishDir, false, false, true)
	if err != nil {
		t.Fatalf("GenerateWithPagesResult failed: %v", err)
	}
	if first.StaticAssets.Copied != 1 || first.StaticAssets.Skipped != 0 {
		t.Fatalf("expected first build to copy one asset, got %#v", first.StaticAssets)
	}
	if first.Pages.Rendered != 4 {
		t.Fatalf("expected first build to render 4 pages, got %#v", first.Pages)
	}
	if first.Pages.Skipped != 0 {
		t.Fatalf("expected first build not to skip pages, got %#v", first.Pages)
	}

	second, err := GenerateWithPagesResult(nil, nil, cfg, cfg.PublishDir, false, false, false)
	if err != nil {
		t.Fatalf("GenerateWithPagesResult second build failed: %v", err)
	}
	if second.StaticAssets.Copied != 0 || second.StaticAssets.Skipped != 1 {
		t.Fatalf("expected second build to skip one current asset, got %#v", second.StaticAssets)
	}
	if second.Pages.Rendered != 0 {
		t.Fatalf("expected second build to render no unchanged pages, got %#v", second.Pages)
	}
	if second.Pages.Skipped != 4 {
		t.Fatalf("expected second build to skip 4 unchanged pages, got %#v", second.Pages)
	}
}
