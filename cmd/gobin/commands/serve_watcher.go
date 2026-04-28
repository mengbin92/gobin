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
	runLoop       func(context.Context, fsWatcher, <-chan fsnotify.Event, <-chan error, serveRuntime, func(func()), func())
	afterFunc     debounceAfterFunc
	rebuild       func(serveRuntime)
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
	})
}

func watchFiles(cfg *config.Config) {
	watchFilesWithRuntime(context.Background(), cfg, serveRuntime{
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		buildDrafts: serveDrafts,
		cleanOutput: serveClean,
		verbose:     serveVerbose,
	})
}

func watchFilesWithRuntime(ctx context.Context, cfg *config.Config, runtime serveRuntime) {
	newServeWatcher(rebuildSiteAndReport).run(ctx, cfg, runtime)
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

func runWatchLoop(ctx context.Context, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func()) {
	runWatchLoopWithWatcher(ctx, nil, events, errors, runtime, schedule, rebuild)
}

func runWatchLoopWithWatcher(ctx context.Context, watcher fsWatcher, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			handleWatchEventWithWatcher(event, watcher, runtime, schedule, rebuild)
		case err, ok := <-errors:
			if !ok {
				return
			}
			fmt.Fprintf(runtime.stderr, "Watcher error: %v\n", err)
		}
	}
}

func handleWatchEvent(event fsnotify.Event, runtime serveRuntime, schedule func(func()), rebuild func()) bool {
	return handleWatchEventWithWatcher(event, nil, runtime, schedule, rebuild)
}

func handleWatchEventWithWatcher(event fsnotify.Event, watcher fsWatcher, runtime serveRuntime, schedule func(func()), rebuild func()) bool {
	if !shouldRebuildForEvent(event) {
		return false
	}
	registerCreatedWatchDirectory(event, watcher, runtime)
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

	paths := []string{cfg.ContentDir, cfg.PageDir, cfg.StaticDir, "templates"}

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
		event.Op&fsnotify.Rename == fsnotify.Rename
}

func addWatchPath(watcher *fsnotify.Watcher, path string) error {
	return addWatchPathFS(&realFSWatcher{Watcher: watcher}, path)
}

func addWatchPathFS(watcher fsWatcher, path string) error {
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
