package commands

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
)

type fsWatcher interface {
	Add(name string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type realFSWatcher struct {
	*fsnotify.Watcher
}

func (w *realFSWatcher) Events() <-chan fsnotify.Event { return w.Watcher.Events }
func (w *realFSWatcher) Errors() <-chan error          { return w.Watcher.Errors }

type serveWatcher struct {
	newWatcher    func() (fsWatcher, error)
	registerPaths func(fsWatcher, *config.Config, serveRuntime) error
	runLoop       func(context.Context, fsWatcher, <-chan fsnotify.Event, <-chan error, serveRuntime, func(func()), func(), func(fsnotify.Event))
	afterFunc     debounceAfterFunc
	rebuild       func(serveRuntime)
	// recordChange, when set, is invoked with every change event that passes
	// shouldRebuildForEvent before the rebuild is debounced. It lets the
	// incremental loader learn which files changed. Nil disables recording.
	recordChange func(fsnotify.Event)
}

func newServeWatcher(rebuild func(serveRuntime)) serveWatcher {
	return serveWatcher{
		newWatcher: func() (fsWatcher, error) {
			w, err := fsnotify.NewWatcher()
			if err != nil {
				return nil, err
			}
			return &realFSWatcher{Watcher: w}, nil
		},
		registerPaths: registerWatchPathsForFSWatcher,
		runLoop:       runWatchLoopWithWatcher,
		afterFunc: func(delay time.Duration, run func()) debounceCancelFunc {
			timer := time.AfterFunc(delay, run)
			return timer.Stop
		},
		rebuild: rebuild,
	}
}

func (w serveWatcher) run(ctx context.Context, cfg *config.Config, runtime serveRuntime) {
	watcher, err := w.newWatcher()
	if err != nil {
		fmt.Fprintf(runtime.stderr, "Failed to create file watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	if err := w.registerPaths(watcher, cfg, runtime); err != nil {
		fmt.Fprintf(runtime.stderr, "Failed to register watch paths: %v\n", err)
		return
	}

	fmt.Fprintln(runtime.stdout, "Watching for file changes...")

	scheduleRebuild := newDebounceScheduler(500*time.Millisecond, w.afterFunc)
	w.runLoop(ctx, watcher, watcher.Events(), watcher.Errors(), runtime, scheduleRebuild, func() {
		if w.rebuild != nil {
			w.rebuild(runtime)
		}
	}, w.recordChange)
}

func watchFiles(cfg *config.Config) {
	watchFilesWithRuntime(context.Background(), cfg, serveRuntime{
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		buildDrafts: serveDrafts,
		cleanOutput: serveClean,
		verbose:     verbose,
	})
}

func watchFilesWithRuntime(ctx context.Context, cfg *config.Config, runtime serveRuntime) {
	// One cache + change set live for the whole watch session. The cache primes
	// itself on the first rebuild (an unprimed cache falls back to a full load),
	// so subsequent saves reparse only the files the watcher reports as changed.
	cache := &contentCache{}
	changes := &changeSet{}
	report := func(reparsed, reused int, full bool) {
		if !runtime.verbose {
			return
		}
		if full {
			fmt.Fprintf(runtime.stdout, "Full reload: %d source(s) parsed\n", reparsed)
			return
		}
		fmt.Fprintf(runtime.stdout, "Partial rebuild: %d changed, %d reused\n", reparsed, reused)
	}
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, report)

	rebuild := func(rt serveRuntime) {
		rebuildSiteAndReportWithResultDeps(rt, func(rt serveRuntime) (*generator.GenerationResult, error) {
			return newServeBuilderWithOptions(loader, generateSite, generateSiteWithResult, generateSiteWithOptions).rebuildResult(rt)
		})
	}

	w := newServeWatcher(rebuild)
	w.recordChange = func(event fsnotify.Event) {
		changes.add(event.Name, classifyChange(event.Name, cfg))
	}
	w.run(ctx, cfg, runtime)
}

func registerWatchPaths(watcher *fsnotify.Watcher, cfg *config.Config, runtime serveRuntime) error {
	if watcher == nil {
		return fmt.Errorf("watcher is nil")
	}

	for _, dir := range watchPaths(cfg) {
		if err := addWatchPath(watcher, dir); err != nil {
			if runtime.verbose {
				fmt.Fprintf(runtime.stdout, "Warning: Could not watch %s: %v\n", dir, err)
			}
		}
	}

	return nil
}

func registerWatchPathsForFSWatcher(watcher fsWatcher, cfg *config.Config, runtime serveRuntime) error {
	if watcher == nil {
		return fmt.Errorf("watcher is nil")
	}

	for _, dir := range watchPaths(cfg) {
		if err := addWatchPathFS(watcher, dir); err != nil {
			if runtime.verbose {
				fmt.Fprintf(runtime.stdout, "Warning: Could not watch %s: %v\n", dir, err)
			}
		}
	}

	return nil
}

func runWatchLoop(ctx context.Context, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func(), record func(fsnotify.Event)) {
	runWatchLoopWithWatcher(ctx, nil, events, errors, runtime, schedule, rebuild, record)
}

func runWatchLoopWithWatcher(ctx context.Context, watcher fsWatcher, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func(), record func(fsnotify.Event)) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			handleWatchEventWithWatcher(event, watcher, runtime, schedule, rebuild, record)
		case err, ok := <-errors:
			if !ok {
				return
			}
			fmt.Fprintf(runtime.stderr, "Watcher error: %v\n", err)
		}
	}
}

func handleWatchEvent(event fsnotify.Event, runtime serveRuntime, schedule func(func()), rebuild func(), record func(fsnotify.Event)) bool {
	return handleWatchEventWithWatcher(event, nil, runtime, schedule, rebuild, record)
}

func handleWatchEventWithWatcher(event fsnotify.Event, watcher fsWatcher, runtime serveRuntime, schedule func(func()), rebuild func(), record func(fsnotify.Event)) bool {
	if !shouldRebuildForEvent(event) {
		return false
	}
	registerCreatedWatchDirectory(event, watcher, runtime)
	if record != nil {
		record(event)
	}
	if runtime.verbose {
		fmt.Fprintf(runtime.stdout, "File changed: %s\n", event.Name)
	}
	schedule(rebuild)
	return true
}

func registerCreatedWatchDirectory(event fsnotify.Event, watcher fsWatcher, runtime serveRuntime) {
	if watcher == nil || event.Op&fsnotify.Create != fsnotify.Create {
		return
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		return
	}
	if !info.IsDir() || shouldSkipWatchDir(info.Name()) {
		return
	}

	if err := addWatchPathFS(watcher, event.Name); err != nil {
		fmt.Fprintf(runtime.stderr, "Warning: Could not watch new directory %s: %v\n", event.Name, err)
	}
}

func newDebounceScheduler(delay time.Duration, afterFunc debounceAfterFunc) func(func()) {
	var cancel debounceCancelFunc
	return func(run func()) {
		if cancel != nil {
			cancel()
		}
		cancel = afterFunc(delay, run)
	}
}

func watchPaths(cfg *config.Config) []string {
	cfg = config.Normalize(cfg)

	paths := []string{
		"config.yaml",
		"config.yml",
		"_config.yml",
		"_config.yaml",
		cfg.ContentDir,
		cfg.PageDir,
		cfg.StaticDir,
		"templates",
	}

	if cfg.Theme != "" {
		paths = append(paths,
			filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts"),
			filepath.Join(cfg.ThemesDir, cfg.Theme, "assets"),
		)
	}

	seen := make(map[string]struct{}, len(paths))
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		filtered = append(filtered, path)
	}

	return filtered
}

func shouldRebuildForEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write ||
		event.Op&fsnotify.Create == fsnotify.Create ||
		event.Op&fsnotify.Rename == fsnotify.Rename ||
		event.Op&fsnotify.Remove == fsnotify.Remove
}

func addWatchPath(watcher *fsnotify.Watcher, path string) error {
	return addWatchPathFS(&realFSWatcher{Watcher: watcher}, path)
}

func addWatchPathFS(watcher fsWatcher, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return watcher.Add(path)
	}

	return filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			name := entry.Name()
			if shouldSkipWatchDir(name) {
				return filepath.SkipDir
			}

			if err := watcher.Add(p); err != nil {
				return err
			}
		}

		return nil
	})
}

func shouldSkipWatchDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "public"
}
