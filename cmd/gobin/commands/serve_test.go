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
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
)

type fakeDevServer struct {
	listenErr      error
	listenCalled   bool
	shutdownCalled bool
	shutdownDone   chan struct{}
	listenRelease  chan struct{}
}

type fakeFSWatcher struct {
	added   []string
	events  chan fsnotify.Event
	errors  chan error
	closeFn func() error
}

func (w *fakeFSWatcher) Add(name string) error {
	w.added = append(w.added, name)
	return nil
}

func (w *fakeFSWatcher) Close() error {
	if w.closeFn != nil {
		return w.closeFn()
	}
	return nil
}

func (w *fakeFSWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *fakeFSWatcher) Errors() <-chan error          { return w.errors }

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
		"config.yaml",
		"config.yml",
		"_config.yml",
		"_config.yaml",
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
		{name: "remove", event: fsnotify.Event{Op: fsnotify.Remove}, want: true},
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

func TestAddWatchPath_AddsFilesDirectly(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("title: Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := addWatchPath(watcher, configPath); err != nil {
		t.Fatalf("addWatchPath failed: %v", err)
	}

	assertContainsPath(t, watcher.WatchList(), configPath)
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

func TestNewDevServerHandler_InjectsLiveReloadIntoHTML(t *testing.T) {
	tmpDir := t.TempDir()
	mustWriteFile(t, filepath.Join(tmpDir, "index.html"), "<html><body>ok</body></html>")

	handler := newDevServerHandler(tmpDir, newLiveReloadBroker())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, liveReloadEndpoint) || !strings.Contains(body, "</script>\n</body>") {
		t.Fatalf("Expected LiveReload script before body close, got %q", body)
	}
}

func TestLiveReloadBroker_SendsReloadEvent(t *testing.T) {
	broker := newLiveReloadBroker()
	req := httptest.NewRequest(http.MethodGet, liveReloadEndpoint, nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := newSyncedResponseRecorder()

	done := make(chan struct{})
	go func() {
		broker.serveEvents(rec, req)
		close(done)
	}()

	waitForCondition(t, time.Second, func() bool {
		return strings.Contains(rec.BodyString(), ": connected")
	})
	broker.notify()
	waitForCondition(t, time.Second, func() bool {
		return strings.Contains(rec.BodyString(), "event: reload")
	})
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected LiveReload stream to close after request cancellation")
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
		watchFiles: func(ctx context.Context, cfg *config.Config, runtime serveRuntime) {
			if ctx == nil {
				t.Fatal("Expected watcher context to be set")
			}
			if runtime.stdout == nil {
				t.Fatal("Expected watcher runtime stdout to be set")
			}
			if runtime.stderr == nil {
				t.Fatal("Expected watcher runtime stderr to be set")
			}
			if !runtime.buildDrafts {
				t.Fatal("Expected watcher runtime to forward buildDrafts")
			}
			if runtime.liveReload == nil {
				t.Fatal("Expected watcher runtime to include LiveReload broker")
			}
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

func TestRunServeWithOps_PrintsStaticAssetStats(t *testing.T) {
	var stdout bytes.Buffer

	err := runServeWithOps(&stdout, false, false, serveOps{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSiteWithResult: func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error) {
			return &generator.GenerationResult{
				Pages:        generator.PageRenderStats{Rendered: 7, Skipped: 2},
				StaticAssets: generator.AssetCopyStats{Copied: 2, Skipped: 3, Deleted: 1},
			}, nil
		},
		watchFiles: func(context.Context, *config.Config, serveRuntime) {
			t.Fatal("watchFiles should not run when watch is disabled")
		},
		startServer: func(io.Writer, *config.Config) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runServeWithOps failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Static assets: copied 2, skipped 3, deleted 1") {
		t.Fatalf("expected static asset stats in serve output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Pages: rendered 7, skipped 2") {
		t.Fatalf("expected page render stats in serve output, got %q", stdout.String())
	}
}

func TestRunServeWithOps_BuildsBeforeStartingServer(t *testing.T) {
	var order []string

	err := runServeWithOps(io.Discard, false, false, serveOps{
		loadSiteInput: func() (*siteBuildInput, error) {
			order = append(order, "load")
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSite: func(*siteBuildInput, string, bool, bool, bool) error {
			order = append(order, "generate")
			return nil
		},
		watchFiles: func(context.Context, *config.Config, serveRuntime) {
			t.Fatal("watchFiles should not run when watch is disabled")
		},
		startServer: func(io.Writer, *config.Config) error {
			order = append(order, "server")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runServeWithOps failed: %v", err)
	}

	want := []string{"load", "generate", "server"}
	if len(order) != len(want) {
		t.Fatalf("Expected order %#v, got %#v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("Expected order %#v, got %#v", want, order)
		}
	}
}

func TestRunServeWithOps_WatcherRuntimeRequestsIncremental(t *testing.T) {
	watchedCh := make(chan serveRuntime, 1)

	err := runServeWithOps(io.Discard, false, true, serveOps{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSite: func(*siteBuildInput, string, bool, bool, bool) error { return nil },
		watchFiles: func(_ context.Context, _ *config.Config, runtime serveRuntime) {
			watchedCh <- runtime
		},
		startServer: func(io.Writer, *config.Config) error { return nil },
	})
	if err != nil {
		t.Fatalf("runServeWithOps failed: %v", err)
	}

	select {
	case runtime := <-watchedCh:
		if !runtime.incremental {
			t.Fatal("expected watcher runtime to have incremental=true")
		}
		if runtime.cleanOutput {
			t.Fatal("expected watcher runtime to have cleanOutput=false")
		}
	case <-time.After(time.Second):
		t.Fatal("watcher was not started")
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
		watchFiles: func(context.Context, *config.Config, serveRuntime) {
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

func TestServeBuilder_InitialBuildForwardsOptions(t *testing.T) {
	var gotOutputDir string
	var gotDrafts bool
	var gotClean bool

	builder := serveBuilder{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSite: func(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
			gotOutputDir = outputDir
			gotDrafts = buildDrafts
			gotClean = cleanOutput
			return nil
		},
	}

	input, err := builder.initialBuild(true, false)
	if err != nil {
		t.Fatalf("initialBuild failed: %v", err)
	}
	if input == nil || input.cfg.PublishDir != "public" {
		t.Fatalf("Expected built input with publish dir, got %#v", input)
	}
	if gotOutputDir != "public" {
		t.Fatalf("Expected outputDir public, got %q", gotOutputDir)
	}
	if !gotDrafts {
		t.Fatal("Expected drafts flag to be forwarded")
	}
	if gotClean {
		t.Fatal("Expected clean flag false to be forwarded")
	}
}

func TestServeBuilder_RebuildUsesIncrementalWhenRuntimeRequests(t *testing.T) {
	var gotOpts generator.GenerationOptions

	builder := serveBuilder{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSiteWithOptionsFn: func(_ *siteBuildInput, opts generator.GenerationOptions) (*generator.GenerationResult, error) {
			gotOpts = opts
			return &generator.GenerationResult{}, nil
		},
	}

	if _, err := builder.rebuildResult(serveRuntime{
		buildDrafts: true,
		cleanOutput: false,
		incremental: true,
	}); err != nil {
		t.Fatalf("rebuildResult failed: %v", err)
	}

	if gotOpts.OutputDir != "public" {
		t.Fatalf("Expected OutputDir public, got %q", gotOpts.OutputDir)
	}
	if !gotOpts.Incremental {
		t.Fatal("Expected Incremental=true to be forwarded")
	}
	if !gotOpts.BuildDrafts {
		t.Fatal("Expected BuildDrafts=true to be forwarded")
	}
	if gotOpts.CleanOutput {
		t.Fatal("Expected CleanOutput=false to be forwarded")
	}
}

func TestServeBuilder_RebuildDisablesIncrementalWhenCleaning(t *testing.T) {
	var gotOpts generator.GenerationOptions

	builder := serveBuilder{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSiteWithOptionsFn: func(_ *siteBuildInput, opts generator.GenerationOptions) (*generator.GenerationResult, error) {
			gotOpts = opts
			return &generator.GenerationResult{}, nil
		},
	}

	// A runtime that asks for both clean output and incremental should drop
	// incremental — clean wipes the manifest, so honoring incremental would
	// be a no-op at best and confusing at worst.
	if _, err := builder.rebuildResult(serveRuntime{
		cleanOutput: true,
		incremental: true,
	}); err != nil {
		t.Fatalf("rebuildResult failed: %v", err)
	}

	if gotOpts.Incremental {
		t.Fatal("Expected Incremental to be disabled when CleanOutput=true")
	}
	if !gotOpts.CleanOutput {
		t.Fatal("Expected CleanOutput=true to be forwarded")
	}
}

func TestServeBuilder_RebuildFallsBackToLegacyPointer(t *testing.T) {
	called := 0

	builder := serveBuilder{
		loadSiteInput: func() (*siteBuildInput, error) {
			return &siteBuildInput{cfg: &config.Config{PublishDir: "public"}}, nil
		},
		generateSiteWithResult: func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error) {
			called++
			return &generator.GenerationResult{}, nil
		},
	}

	if _, err := builder.rebuildResult(serveRuntime{}); err != nil {
		t.Fatalf("rebuildResult failed: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected legacy generateSiteWithResult to run once, got %d", called)
	}
}

func TestServeWatcher_RunDelegatesToWatchLoop(t *testing.T) {
	called := false
	watcher := serveWatcher{
		registerPaths: func(fsWatcher, *config.Config, serveRuntime) error { return nil },
		newWatcher: func() (fsWatcher, error) {
			return &fakeFSWatcher{
				events: make(chan fsnotify.Event),
				errors: make(chan error),
				closeFn: func() error {
					return nil
				},
			}, nil
		},
		runLoop: func(ctx context.Context, watcher fsWatcher, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func(), record func(fsnotify.Event)) {
			called = true
		},
		afterFunc: func(delay time.Duration, run func()) debounceCancelFunc {
			return func() bool { return true }
		},
	}

	watcher.run(context.Background(), &config.Config{}, serveRuntime{
		stdout: io.Discard,
		stderr: io.Discard,
	})

	if !called {
		t.Fatal("Expected watcher run loop to be called")
	}
}

func TestServeServer_RunDelegatesToLifecycle(t *testing.T) {
	server := serveServer{
		addr: ":9000",
		newServer: func(addr string, cfg *config.Config, runtime serveRuntime) devServer {
			return &fakeDevServer{listenErr: http.ErrServerClosed}
		},
		signals: make(chan os.Signal),
		runLifecycle: func(stdout io.Writer, server devServer, addr string, signals <-chan os.Signal) error {
			if addr != ":9000" {
				t.Fatalf("Expected addr :9000, got %q", addr)
			}
			return nil
		},
	}

	if err := server.run(io.Discard, &config.Config{PublishDir: "public"}); err != nil {
		t.Fatalf("serveServer.run failed: %v", err)
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

func TestBuildSiteWithDeps_ForwardsFlags(t *testing.T) {
	var gotDrafts bool
	var gotClean bool
	err := buildSiteWithDeps(func() (*siteBuildInput, error) {
		return &siteBuildInput{
			cfg: &config.Config{PublishDir: "public"},
		}, nil
	}, func(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
		gotDrafts = buildDrafts
		gotClean = cleanOutput
		return nil
	}, true, false)
	if err != nil {
		t.Fatalf("buildSiteWithOptions failed: %v", err)
	}
	if !gotDrafts {
		t.Fatal("Expected drafts flag to be forwarded")
	}
	if gotClean {
		t.Fatal("Expected clean flag to be forwarded as false")
	}
}

func TestRegisterWatchPaths_UsesProvidedOutputsOnWatcherFailure(t *testing.T) {
	cfg := &config.Config{
		ContentDir: "missing-content",
		PageDir:    "missing-pages",
		StaticDir:  "missing-assets",
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	defer watcher.Close()

	err = registerWatchPaths(watcher, cfg, serveRuntime{
		stdout:  &stdout,
		stderr:  &stderr,
		verbose: true,
	})
	if err != nil {
		t.Fatalf("registerWatchPaths failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Warning: Could not watch missing-content") {
		t.Fatalf("Expected watch warning on provided stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Expected no stderr output for watch path warnings, got %q", stderr.String())
	}
}

func TestRunWatchLoop_SchedulesRebuildFromEvents(t *testing.T) {
	events := make(chan fsnotify.Event, 1)
	errorsCh := make(chan error)
	done := make(chan struct{})
	var scheduled bool
	var rebuilt bool

	go func() {
		runWatchLoop(context.Background(), events, errorsCh, serveRuntime{
			stdout:  io.Discard,
			stderr:  io.Discard,
			verbose: true,
		}, func(run func()) {
			scheduled = true
			run()
		}, func() {
			rebuilt = true
		}, nil)
		close(done)
	}()

	events <- fsnotify.Event{Name: "content/post.md", Op: fsnotify.Write}
	close(events)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected watch loop to exit after events channel closed")
	}

	if !scheduled {
		t.Fatal("Expected runWatchLoop to schedule rebuild")
	}
	if !rebuilt {
		t.Fatal("Expected scheduled rebuild callback to run")
	}
}

func TestServeWatchLoop_RebuildFailureDoesNotStopLaterRebuild(t *testing.T) {
	events := make(chan fsnotify.Event, 2)
	errorsCh := make(chan error)
	done := make(chan struct{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var rebuildCalls int

	go func() {
		runWatchLoop(context.Background(), events, errorsCh, serveRuntime{
			stdout: &stdout,
			stderr: &stderr,
		}, func(run func()) {
			run()
		}, func() {
			rebuildCalls++
			call := rebuildCalls
			rebuildSiteAndReportWithDeps(serveRuntime{
				stdout: &stdout,
				stderr: &stderr,
			}, func(serveRuntime) error {
				if call == 1 {
					return errors.New("first rebuild failed")
				}
				return nil
			})
		}, nil)
		close(done)
	}()

	events <- fsnotify.Event{Name: "content/first.md", Op: fsnotify.Write}
	events <- fsnotify.Event{Name: "content/second.md", Op: fsnotify.Write}
	close(events)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected watch loop to exit after events channel closed")
	}

	if rebuildCalls != 2 {
		t.Fatalf("Expected two rebuild attempts, got %d", rebuildCalls)
	}
	if !strings.Contains(stderr.String(), "Error rebuilding site: first rebuild failed") {
		t.Fatalf("Expected first rebuild failure output, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Site rebuilt successfully in") {
		t.Fatalf("Expected later rebuild success output, got %q", stdout.String())
	}
}

func TestServeRebuildLifecycle_UsesLatestFilesAndRecoversAfterFailure(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	defer os.Chdir(oldWd)
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runtime := serveRuntime{
		stdout:      &stdout,
		stderr:      &stderr,
		buildDrafts: false,
		cleanOutput: true,
	}

	if _, err := buildSiteWithResultWithDeps(loadSiteBuildInput, generateSiteWithResult, false, true); err != nil {
		t.Fatalf("initial build failed: %v", err)
	}
	assertFileContains(t, filepath.Join(siteDir, "public", "index.html"), "Initial Title")

	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2026-05-01-initial.md"), `---
title: "Broken Title"
date: 2026-05-01T10:00:00+08:00
---

Broken content`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), `{{ define "singleMain" }}{{ .Missing.Field }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)

	rebuildSiteAndReport(runtime)
	if !strings.Contains(stderr.String(), "Error rebuilding site:") {
		t.Fatalf("Expected failed rebuild to be reported, got %q", stderr.String())
	}

	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2026-05-01-initial.md"), `---
title: "Recovered Title"
date: 2026-05-01T10:00:00+08:00
---

Recovered content`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), serveFixtureSingleTemplate())

	rebuildSiteAndReport(runtime)
	assertFileContains(t, filepath.Join(siteDir, "public", "index.html"), "Recovered Title")
	assertFileContains(t, filepath.Join(siteDir, "public", "initial", "index.html"), "Recovered content")
	if !strings.Contains(stdout.String(), "Site rebuilt successfully in") {
		t.Fatalf("Expected successful rebuild output, got %q", stdout.String())
	}
}

func TestRunWatchLoop_WritesWatcherErrors(t *testing.T) {
	events := make(chan fsnotify.Event)
	errorsCh := make(chan error, 1)
	done := make(chan struct{})
	var stderr bytes.Buffer

	go func() {
		runWatchLoop(context.Background(), events, errorsCh, serveRuntime{
			stdout: io.Discard,
			stderr: &stderr,
		}, func(func()) {}, func() {}, nil)
		close(done)
	}()

	errorsCh <- errors.New("watch failed")
	close(errorsCh)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected watch loop to exit after errors channel closed")
	}

	if !strings.Contains(stderr.String(), "Watcher error: watch failed") {
		t.Fatalf("Expected watcher error output, got %q", stderr.String())
	}
}

func TestRunWatchLoop_ExitsWhenErrorsChannelClosesFirst(t *testing.T) {
	events := make(chan fsnotify.Event)
	errorsCh := make(chan error)
	done := make(chan struct{})

	go func() {
		runWatchLoop(context.Background(), events, errorsCh, serveRuntime{
			stdout: io.Discard,
			stderr: io.Discard,
		}, func(func()) {}, func() {}, nil)
		close(done)
	}()

	close(errorsCh)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected watch loop to exit when errors channel closes")
	}
}

func TestRunWatchLoop_ExitsWhenContextCancelled(t *testing.T) {
	events := make(chan fsnotify.Event)
	errorsCh := make(chan error)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		runWatchLoop(ctx, events, errorsCh, serveRuntime{
			stdout: io.Discard,
			stderr: io.Discard,
		}, func(func()) {}, func() {}, nil)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Expected watch loop to exit when context is cancelled")
	}
}

func TestHandleWatchEvent_SchedulesRebuildForSupportedEvents(t *testing.T) {
	var stdout bytes.Buffer
	var scheduled bool
	var rebuilt bool

	triggered := handleWatchEvent(fsnotify.Event{
		Name: "content/post.md",
		Op:   fsnotify.Write,
	}, serveRuntime{
		stdout:  &stdout,
		stderr:  io.Discard,
		verbose: true,
	}, func(run func()) {
		scheduled = true
		run()
	}, func() {
		rebuilt = true
	}, nil)

	if !triggered {
		t.Fatal("Expected write event to trigger rebuild scheduling")
	}
	if !scheduled {
		t.Fatal("Expected rebuild to be scheduled")
	}
	if !rebuilt {
		t.Fatal("Expected scheduled rebuild callback to run")
	}
	if !strings.Contains(stdout.String(), "File changed: content/post.md") {
		t.Fatalf("Expected verbose watch output, got %q", stdout.String())
	}
}

func TestHandleWatchEventWithWatcher_RegistersCreatedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "content", "posts")
	mustMkdirAll(t, filepath.Join(newDir, "nested"))

	fakeWatcher := &fakeFSWatcher{}
	var scheduled bool

	triggered := handleWatchEventWithWatcher(fsnotify.Event{
		Name: newDir,
		Op:   fsnotify.Create,
	}, fakeWatcher, serveRuntime{
		stdout:  io.Discard,
		stderr:  io.Discard,
		verbose: true,
	}, func(run func()) {
		scheduled = true
	}, func() {}, nil)

	if !triggered {
		t.Fatal("Expected created directory event to trigger rebuild scheduling")
	}
	if !scheduled {
		t.Fatal("Expected rebuild to be scheduled")
	}
	assertContainsPath(t, fakeWatcher.added, newDir)
	assertContainsPath(t, fakeWatcher.added, filepath.Join(newDir, "nested"))
}

func TestHandleWatchEvent_IgnoresUnsupportedEvents(t *testing.T) {
	var scheduled bool
	triggered := handleWatchEvent(fsnotify.Event{
		Name: "content/post.md",
		Op:   fsnotify.Chmod,
	}, serveRuntime{
		stdout:  io.Discard,
		stderr:  io.Discard,
		verbose: true,
	}, func(func()) {
		scheduled = true
	}, func() {}, nil)

	if triggered {
		t.Fatal("Expected chmod event to be ignored")
	}
	if scheduled {
		t.Fatal("Expected ignored event not to schedule rebuild")
	}
}

func TestNewDebounceScheduler_CancelsPreviousTimer(t *testing.T) {
	var stopCalls int
	var scheduled []func()
	scheduler := newDebounceScheduler(500*time.Millisecond, func(delay time.Duration, run func()) debounceCancelFunc {
		if delay != 500*time.Millisecond {
			t.Fatalf("Expected debounce delay 500ms, got %v", delay)
		}
		scheduled = append(scheduled, run)
		return func() bool {
			stopCalls++
			return true
		}
	})

	scheduler(func() {})
	scheduler(func() {})

	if len(scheduled) != 2 {
		t.Fatalf("Expected two scheduled callbacks, got %d", len(scheduled))
	}
	if stopCalls != 1 {
		t.Fatalf("Expected previous debounce timer to be stopped once, got %d", stopCalls)
	}
}

func TestRebuildSiteAndReport_Success(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rebuildSiteAndReportWithDeps(serveRuntime{
		stdout:      &stdout,
		stderr:      &stderr,
		buildDrafts: true,
		cleanOutput: false,
	}, func(serveRuntime) error {
		return nil
	})

	if !strings.Contains(stdout.String(), "Rebuilding site...") {
		t.Fatalf("Expected rebuild start output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Site rebuilt successfully in") {
		t.Fatalf("Expected rebuild success output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Expected no stderr output on rebuild success, got %q", stderr.String())
	}
}

func TestRebuildSiteAndReport_NotifiesLiveReloadOnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	broker := newLiveReloadBroker()
	events, unsubscribe := broker.subscribe()
	defer unsubscribe()

	rebuildSiteAndReportWithDeps(serveRuntime{
		stdout:     &stdout,
		stderr:     &stderr,
		liveReload: broker,
	}, func(serveRuntime) error {
		return nil
	})

	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("Expected rebuild success to notify LiveReload clients")
	}
}

func TestRebuildSiteAndReport_PrintsStaticAssetStats(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rebuildSiteAndReportWithResultDeps(serveRuntime{
		stdout:      &stdout,
		stderr:      &stderr,
		buildDrafts: true,
		cleanOutput: false,
	}, func(serveRuntime) (*generator.GenerationResult, error) {
		return &generator.GenerationResult{
			Pages:        generator.PageRenderStats{Rendered: 5, Skipped: 6},
			StaticAssets: generator.AssetCopyStats{Copied: 1, Skipped: 4, Deleted: 2},
		}, nil
	})

	if !strings.Contains(stdout.String(), "Static assets: copied 1, skipped 4, deleted 2") {
		t.Fatalf("Expected rebuild output to contain asset stats, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Pages: rendered 5, skipped 6") {
		t.Fatalf("Expected rebuild output to contain page render stats, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Expected no stderr output on rebuild success, got %q", stderr.String())
	}
}

func TestRebuildSiteAndReport_Failure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rebuildSiteAndReportWithDeps(serveRuntime{
		stdout:      &stdout,
		stderr:      &stderr,
		buildDrafts: true,
		cleanOutput: false,
	}, func(serveRuntime) error {
		return errors.New("boom")
	})

	if !strings.Contains(stdout.String(), "Rebuilding site...") {
		t.Fatalf("Expected rebuild start output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error rebuilding site: boom") {
		t.Fatalf("Expected rebuild failure output, got %q", stderr.String())
	}
}

func TestRebuildSiteAndReport_DoesNotNotifyLiveReloadOnFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	broker := newLiveReloadBroker()
	events, unsubscribe := broker.subscribe()
	defer unsubscribe()

	rebuildSiteAndReportWithDeps(serveRuntime{
		stdout:     &stdout,
		stderr:     &stderr,
		liveReload: broker,
	}, func(serveRuntime) error {
		return errors.New("boom")
	})

	select {
	case <-events:
		t.Fatal("Expected failed rebuild not to notify LiveReload clients")
	case <-time.After(20 * time.Millisecond):
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", path, err)
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

func writeServeFixtureSite(t *testing.T, siteDir string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(siteDir, "config.yaml"), `title: Test Site
description: Test Description
baseURL: https://example.com
contentDir: _posts
pageDir: pages
staticDir: assets
publishDir: public
outputs:
  feed: false
  sitemap: false
  search: false
  robots: false
`)
	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2026-05-01-initial.md"), `---
title: "Initial Title"
date: 2026-05-01T10:00:00+08:00
---

Initial content`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ render .MainTemplate . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "list.html"), `{{ define "listMain" }}{{ range .Posts }}{{ .Title }}{{ end }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), serveFixtureSingleTemplate())
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "header.html"), `{{ define "header" }}{{ end }}{{ define "headerNested" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "footer.html"), `{{ define "footer" }}{{ end }}{{ define "footerNested" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "comments.html"), `{{ define "comments" }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "partials", "analytics.html"), `{{ define "analytics" }}{{ end }}`)
}

func serveFixtureSingleTemplate() string {
	return `{{ define "singleMain" }}{{ .Post.Title }} {{ safeHTML .Post.ContentHTML }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`
}

func assertFileContains(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("Expected %s to contain %q, got %s", path, expected, string(content))
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Timed out waiting for condition")
}

type syncedResponseRecorder struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	status  int
	flushed int
}

func newSyncedResponseRecorder() *syncedResponseRecorder {
	return &syncedResponseRecorder{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (r *syncedResponseRecorder) Header() http.Header { return r.header }

func (r *syncedResponseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(p)
}

func (r *syncedResponseRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = statusCode
}

func (r *syncedResponseRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushed++
}

func (r *syncedResponseRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}
