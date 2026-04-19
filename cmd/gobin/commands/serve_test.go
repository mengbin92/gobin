package commands

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
)

type fakeDevServer struct {
	listenErr      error
	listenCalled   bool
	shutdownCalled bool
	shutdownDone   chan struct{}
	listenRelease  chan struct{}
}

func (s *fakeDevServer) ListenAndServe() error {
	s.listenCalled = true
	if s.listenRelease != nil {
		<-s.listenRelease
	}
	return s.listenErr
}

func (s *fakeDevServer) Shutdown(context.Context) error {
	s.shutdownCalled = true
	if s.shutdownDone != nil {
		close(s.shutdownDone)
	}
	return nil
}

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
		"pages",
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

func TestRunServeWithOps_StartsWatcherAndServer(t *testing.T) {
	watchedCh := make(chan struct{}, 1)
	var started bool
	var stdout bytes.Buffer

	err := runServeWithOps(&stdout, true, true, serveOps{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{
				cfg: &config.Config{PublishDir: "public"},
			}, nil
		},
		generateSite: func(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
			if outputDir != "public" {
				t.Fatalf("Expected publish dir public, got %q", outputDir)
			}
			if !buildDrafts {
				t.Fatal("Expected buildDrafts to be forwarded")
			}
			return nil
		},
		watchFiles: func(cfg *config.Config) {
			watchedCh <- struct{}{}
		},
		startServer: func(stdout io.Writer, cfg *config.Config) error {
			started = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runServeWithOps failed: %v", err)
	}

	select {
	case <-watchedCh:
	case <-time.After(time.Second):
		t.Fatal("Expected watcher to be started")
	}
	if !started {
		t.Fatal("Expected server to be started")
	}
	output := stdout.String()
	if !strings.Contains(output, "Building site...") || !strings.Contains(output, "Site built successfully!") {
		t.Fatalf("Unexpected serve output: %q", output)
	}
}

func TestRunServeWithOps_BuildFailureStopsBeforeServer(t *testing.T) {
	buildErr := errors.New("boom")
	var started bool

	err := runServeWithOps(io.Discard, false, true, serveOps{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSite: func(*siteBuildInput, string, bool, bool, bool) error {
			return buildErr
		},
		watchFiles: func(*config.Config) {
			t.Fatal("watchFiles should not run on build failure")
		},
		startServer: func(io.Writer, *config.Config) error {
			started = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "build site: boom") {
		t.Fatalf("Expected wrapped build error, got %v", err)
	}
	if started {
		t.Fatal("startServer should not run on build failure")
	}
}

func TestRunServerLifecycle_GracefulShutdown(t *testing.T) {
	signals := make(chan os.Signal, 1)
	server := &fakeDevServer{
		listenErr:     http.ErrServerClosed,
		listenRelease: make(chan struct{}),
		shutdownDone:  make(chan struct{}),
	}
	var stdout bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServerLifecycle(&stdout, server, ":8080", signals)
	}()

	signals <- os.Interrupt
	<-server.shutdownDone
	close(server.listenRelease)

	if err := <-done; err != nil {
		t.Fatalf("runServerLifecycle failed: %v", err)
	}
	if !server.listenCalled {
		t.Fatal("Expected server ListenAndServe to be called")
	}
	if !server.shutdownCalled {
		t.Fatal("Expected server Shutdown to be called")
	}
	output := stdout.String()
	if !strings.Contains(output, "Development server started at http://localhost:8080") {
		t.Fatalf("Expected startup output, got %q", output)
	}
	if !strings.Contains(output, "Shutting down server...") {
		t.Fatalf("Expected shutdown output, got %q", output)
	}
}

func TestRunServerLifecycle_ReturnsServerError(t *testing.T) {
	server := &fakeDevServer{listenErr: errors.New("listen failed")}

	err := runServerLifecycle(io.Discard, server, ":8080", make(chan os.Signal))
	if err == nil || !strings.Contains(err.Error(), "server error: listen failed") {
		t.Fatalf("Expected wrapped server error, got %v", err)
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
