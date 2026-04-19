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
	watchFiles    func(*config.Config)
}

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
		watchFiles:    watchFiles,
	})
}

func runServeWithOps(stdout io.Writer, buildDrafts bool, watch bool, ops serveOps) error {
	input, err := ops.loadSiteInput()
	if err != nil {
		return err
	}
	cfg := input.cfg

	fmt.Fprintln(stdout, "Building site...")
	if err := ops.generateSite(input, cfg.PublishDir, false, buildDrafts, serveClean); err != nil {
		return fmt.Errorf("build site: %w", err)
	}
	fmt.Fprintln(stdout, "Site built successfully!")

	if watch {
		go ops.watchFiles(cfg)
	}

	return ops.startServer(stdout, cfg)
}

// buildSite rebuilds the site from the current working directory configuration.
func buildSite(buildDrafts bool) error {
	input, err := loadSiteBuildInput()
	if err != nil {
		return err
	}
	return generateSite(input, input.cfg.PublishDir, false, buildDrafts, serveClean)
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

// watchFiles watches for file changes and rebuilds the site
func watchFiles(cfg *config.Config) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	for _, dir := range watchPaths(cfg) {
		if err := addWatchPath(watcher, dir); err != nil {
			if serveVerbose {
				fmt.Printf("Warning: Could not watch %s: %v\n", dir, err)
			}
		}
	}

	fmt.Println("Watching for file changes...")

	// Debounce timer
	var timer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if shouldRebuildForEvent(event) {

				if serveVerbose {
					fmt.Printf("File changed: %s\n", event.Name)
				}

				// Cancel existing timer
				if timer != nil {
					timer.Stop()
				}

				// Set new timer for debouncing
				timer = time.AfterFunc(debounceDuration, func() {
					fmt.Println("\nRebuilding site...")
					start := time.Now()

					if err := buildSite(serveDrafts); err != nil {
						fmt.Fprintf(os.Stderr, "Error rebuilding site: %v\n", err)
						return
					}

					elapsed := time.Since(start)
					fmt.Printf("Site rebuilt successfully in %v\n", elapsed)
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
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
