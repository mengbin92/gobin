package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
)

func TestWatchPaths_DefaultAndTheme(t *testing.T) {
	cfg := &config.Config{
		ContentDir: "content",
		StaticDir:  "assets",
		ThemesDir:  "themes",
		Theme:      "official-website",
	}

	got := watchPaths(cfg)
	want := []string{
		"content",
		"assets",
		"templates",
		filepath.Join("themes", "official-website", "layouts"),
		filepath.Join("themes", "official-website", "assets"),
	}

	if len(got) != len(want) {
		t.Fatalf("Expected %d watch paths, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Expected watch path %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestShouldRebuildForEvent(t *testing.T) {
	tests := []struct {
		name  string
		event fsnotify.Event
		want  bool
	}{
		{name: "write", event: fsnotify.Event{Op: fsnotify.Write}, want: true},
		{name: "create", event: fsnotify.Event{Op: fsnotify.Create}, want: true},
		{name: "rename", event: fsnotify.Event{Op: fsnotify.Rename}, want: true},
		{name: "remove", event: fsnotify.Event{Op: fsnotify.Remove}, want: false},
		{name: "chmod", event: fsnotify.Event{Op: fsnotify.Chmod}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRebuildForEvent(tt.event); got != tt.want {
				t.Fatalf("Expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestShouldSkipWatchDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: ".git", want: true},
		{name: "node_modules", want: true},
		{name: "public", want: true},
		{name: "content", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipWatchDir(tt.name); got != tt.want {
				t.Fatalf("Expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestAddWatchPath_SkipsIgnoredDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	mustMkdirAll(t, filepath.Join(tmpDir, "content", ".git"))
	mustMkdirAll(t, filepath.Join(tmpDir, "content", "node_modules"))
	mustMkdirAll(t, filepath.Join(tmpDir, "content", "public"))
	mustMkdirAll(t, filepath.Join(tmpDir, "content", "posts"))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := addWatchPath(watcher, filepath.Join(tmpDir, "content")); err != nil {
		t.Fatalf("addWatchPath failed: %v", err)
	}

	watched := watcher.WatchList()
	assertContainsPath(t, watched, filepath.Join(tmpDir, "content"))
	assertContainsPath(t, watched, filepath.Join(tmpDir, "content", "posts"))
	assertNotContainsPath(t, watched, filepath.Join(tmpDir, "content", ".git"))
	assertNotContainsPath(t, watched, filepath.Join(tmpDir, "content", "node_modules"))
	assertNotContainsPath(t, watched, filepath.Join(tmpDir, "content", "public"))
}

func TestNewDevServerHandler_ServesFilesWithoutLiveReloadEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	indexPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>ok</body></html>"), 0644); err != nil {
		t.Fatalf("Failed to write index file: %v", err)
	}

	handler := newDevServerHandler(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "<html><body>ok</body></html>" {
		t.Fatalf("Unexpected body: %s", body)
	}

	liveReloadReq := httptest.NewRequest(http.MethodGet, "/_livereload", nil)
	liveReloadRec := httptest.NewRecorder()
	handler.ServeHTTP(liveReloadRec, liveReloadReq)

	if liveReloadRec.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404 for /_livereload, got %d", liveReloadRec.Code)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

func assertContainsPath(t *testing.T, paths []string, target string) {
	t.Helper()
	for _, path := range paths {
		if path == target {
			return
		}
	}
	t.Fatalf("Expected watch list to contain %s, got %#v", target, paths)
}

func assertNotContainsPath(t *testing.T, paths []string, target string) {
	t.Helper()
	for _, path := range paths {
		if path == target {
			t.Fatalf("Expected watch list to exclude %s, got %#v", target, paths)
		}
	}
}
