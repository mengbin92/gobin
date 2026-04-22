package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
	"github.com/spf13/cobra"
)

type devServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type serveOps struct {
	loadSiteInput func() (*siteBuildInput, error)
	generateSite  func(*siteBuildInput, string, bool, bool, bool) error
	startServer   func(io.Writer, *config.Config) error
	watchFiles    func(context.Context, *config.Config, serveRuntime)
}

type serveRuntime struct {
	stdout      io.Writer
	stderr      io.Writer
	buildDrafts bool
	cleanOutput bool
	verbose     bool
}

type debounceCancelFunc func() bool
type debounceAfterFunc func(time.Duration, func()) debounceCancelFunc

var (
	servePort    int
	serveWatch   bool
	serveVerbose bool
	serveDrafts  bool
	serveClean   bool
)

// ServeCmd is the serve command
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	Long: `Start a local development server with optional file watching and automatic rebuild.

The server watches for changes in your content and templates and automatically
rebuilds the site when files change.

The current serve workflow does not inject a LiveReload script into pages.
After a rebuild, refresh the browser manually to see the latest output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe(cmd.OutOrStdout(), serveDrafts, serveWatch)
	},
}

func init() {
	ServeCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to serve on")
	ServeCmd.Flags().BoolVarP(&serveWatch, "watch", "w", true, "Watch for file changes and rebuild")
	ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Verbose output")
	ServeCmd.Flags().BoolVar(&serveDrafts, "drafts", false, "Include draft posts in the output")
	ServeCmd.Flags().BoolVar(&serveClean, "clean", true, "Clean the output directory before rebuilding")
}

func runServe(stdout io.Writer, buildDrafts bool, watch bool) error {
	return runServeWithOps(stdout, buildDrafts, watch, serveOps{
		loadSiteInput: loadSiteBuildInput,
		generateSite:  generateSite,
		startServer:   startServer,
		watchFiles:    watchFilesWithRuntime,
	})
}

func runServeWithOps(stdout io.Writer, buildDrafts bool, watch bool, ops serveOps) error {
	input, err := ops.loadSiteInput()
	if err != nil {
		return err
	}
	cfg := input.cfg
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()

	fmt.Fprintln(stdout, "Building site...")
	if err := ops.generateSite(input, cfg.PublishDir, false, buildDrafts, serveClean); err != nil {
		return fmt.Errorf("build site: %w", err)
	}
	fmt.Fprintln(stdout, "Site built successfully!")

	if watch {
		go ops.watchFiles(watchCtx, cfg, serveRuntime{
			stdout:      stdout,
			stderr:      os.Stderr,
			buildDrafts: buildDrafts,
			cleanOutput: serveClean,
			verbose:     serveVerbose,
		})
	}

	err = ops.startServer(stdout, cfg)
	cancelWatch()
	return err
}

// buildSite rebuilds the site from the current working directory configuration.
func buildSite(buildDrafts bool) error {
	return buildSiteWithOptions(buildDrafts, serveClean)
}

func buildSiteWithOptions(buildDrafts bool, cleanOutput bool) error {
	return buildSiteWithDeps(loadSiteBuildInput, generateSite, buildDrafts, cleanOutput)
}

func buildSiteWithDeps(loadInput func() (*siteBuildInput, error), generate func(*siteBuildInput, string, bool, bool, bool) error, buildDrafts bool, cleanOutput bool) error {
	if loadInput == nil {
		return fmt.Errorf("load site input function is nil")
	}
	if generate == nil {
		return fmt.Errorf("generate site function is nil")
	}

	input, err := loadInput()
	if err != nil {
		return err
	}
	return generate(input, input.cfg.PublishDir, false, buildDrafts, cleanOutput)
}

func buildSiteWithRuntime(runtime serveRuntime) error {
	return buildSiteWithDeps(loadSiteBuildInput, generateSite, runtime.buildDrafts, runtime.cleanOutput)
}

// startServer starts the HTTP development server
func startServer(stdout io.Writer, cfg *config.Config) error {
	addr := fmt.Sprintf(":%d", servePort)
	server := newDevServer(addr, cfg.PublishDir)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	return runServerLifecycle(stdout, server, addr, sigChan)
}

func runServerLifecycle(stdout io.Writer, server devServer, addr string, signals <-chan os.Signal) error {
	go func() {
		<-signals

		fmt.Fprintln(stdout, "\nShutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	fmt.Fprintf(stdout, "Development server started at http://localhost%s\n", addr)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newDevServer(addr, outputDir string) devServer {
	return &http.Server{
		Addr:         addr,
		Handler:      newDevServerHandler(outputDir),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func newDevServerHandler(outputDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(outputDir)))
	return mux
}

// watchFiles watches for file changes and rebuilds the site.
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(runtime.stderr, "Failed to create file watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	if err := registerWatchPaths(watcher, cfg, runtime); err != nil {
		fmt.Fprintf(runtime.stderr, "Failed to register watch paths: %v\n", err)
		return
	}

	fmt.Fprintln(runtime.stdout, "Watching for file changes...")

	scheduleRebuild := newDebounceScheduler(500*time.Millisecond, func(delay time.Duration, run func()) debounceCancelFunc {
		timer := time.AfterFunc(delay, run)
		return timer.Stop
	})
	runWatchLoop(ctx, watcher.Events, watcher.Errors, runtime, scheduleRebuild, func() {
		rebuildSiteAndReport(runtime)
	})
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

func runWatchLoop(ctx context.Context, events <-chan fsnotify.Event, errors <-chan error, runtime serveRuntime, schedule func(func()), rebuild func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			handleWatchEvent(event, runtime, schedule, rebuild)
		case err, ok := <-errors:
			if !ok {
				return
			}
			fmt.Fprintf(runtime.stderr, "Watcher error: %v\n", err)
		}
	}
}

func rebuildSiteAndReport(runtime serveRuntime) {
	rebuildSiteAndReportWithDeps(runtime, func(runtime serveRuntime) error {
		return buildSiteWithRuntime(runtime)
	})
}

func rebuildSiteAndReportWithDeps(runtime serveRuntime, rebuild func(serveRuntime) error) {
	if rebuild == nil {
		fmt.Fprintln(runtime.stderr, "Error rebuilding site: rebuild function is nil")
		return
	}
	fmt.Fprintln(runtime.stdout, "\nRebuilding site...")
	start := time.Now()

	if err := rebuild(runtime); err != nil {
		fmt.Fprintf(runtime.stderr, "Error rebuilding site: %v\n", err)
		return
	}

	elapsed := time.Since(start)
	fmt.Fprintf(runtime.stdout, "Site rebuilt successfully in %v\n", elapsed)
}

func handleWatchEvent(event fsnotify.Event, runtime serveRuntime, schedule func(func()), rebuild func()) bool {
	if !shouldRebuildForEvent(event) {
		return false
	}
	if runtime.verbose {
		fmt.Fprintf(runtime.stdout, "File changed: %s\n", event.Name)
	}
	schedule(rebuild)
	return true
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

// addWatchPath adds a path and all its subdirectories to the watcher
func addWatchPath(watcher *fsnotify.Watcher, path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and node_modules
		if info.IsDir() {
			name := info.Name()
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
