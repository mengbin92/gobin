package generator

import (
	"os"
	"path/filepath"
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
