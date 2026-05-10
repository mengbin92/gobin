package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_RejectsInvalidBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"://bad\"\n")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected invalid baseURL to fail validation")
	}
	if !strings.Contains(err.Error(), "baseURL") {
		t.Fatalf("Expected baseURL validation error, got: %v", err)
	}
}

func TestLoadConfig_RejectsInvalidPostsPermalink(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"https://example.com\"\npermalinks:\n  posts: \"posts/:title\"\n")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected invalid posts permalink to fail validation")
	}
	if !strings.Contains(err.Error(), "permalinks.posts") {
		t.Fatalf("Expected permalink validation error, got: %v", err)
	}
}

func TestLoadConfig_RejectsPostsPermalinkTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"https://example.com\"\npermalinks:\n  posts: \"/../:slug/\"\n")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected path traversal posts permalink to fail validation")
	}
	if !strings.Contains(err.Error(), "permalinks.posts") {
		t.Fatalf("Expected permalink validation error, got: %v", err)
	}
}

func TestLoadConfig_RejectsMissingThemeDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"https://example.com\"\ntheme: \"demo\"\nthemesDir: \"themes\"\n")

	oldWd := mustChdir(t, tmpDir)
	defer mustRestoreWd(t, oldWd)

	_, err := Load("config.yaml")
	if err == nil {
		t.Fatal("Expected missing theme directory to fail validation")
	}
	if !strings.Contains(err.Error(), "theme") {
		t.Fatalf("Expected theme validation error, got: %v", err)
	}
}

func TestLoadConfig_RejectsPublishDirOverlappingContentDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"https://example.com\"\ncontentDir: \"public\"\npublishDir: \"public\"\n")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected overlapping publishDir and contentDir to fail validation")
	}
	if !strings.Contains(err.Error(), "publishDir") {
		t.Fatalf("Expected publishDir validation error, got: %v", err)
	}
}

func TestLoadConfig_RejectsPublishDirOutsideSiteRoot(t *testing.T) {
	tests := []struct {
		name       string
		publishDir string
	}{
		{name: "parent traversal", publishDir: "../public"},
		{name: "current directory", publishDir: "."},
		{name: "absolute path", publishDir: filepath.Join(t.TempDir(), "public")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			mustWriteConfigFile(t, configPath, "title: Test\nbaseURL: \"https://example.com\"\npublishDir: \""+filepath.ToSlash(tt.publishDir)+"\"\n")

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("Expected unsafe publishDir to fail validation")
			}
			if !strings.Contains(err.Error(), "publishDir") {
				t.Fatalf("Expected publishDir validation error, got: %v", err)
			}
		})
	}
}

func mustWriteConfigFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
}

func mustChdir(t *testing.T, dir string) string {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	return oldWd
}

func mustRestoreWd(t *testing.T, oldWd string) {
	t.Helper()
	if err := os.Chdir(oldWd); err != nil {
		t.Fatalf("Failed to restore cwd: %v", err)
	}
}
