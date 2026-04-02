package commands

import (
	"context"
	"fmt"
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
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/spf13/cobra"
)

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
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.LoadDefault()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Build initial site
		fmt.Println("Building site...")
		if err := buildSite(cfg, serveDrafts); err != nil {
			fmt.Fprintf(os.Stderr, "Error building site: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Site built successfully!")

		// Start file watcher if enabled
		if serveWatch {
			go watchFiles(cfg)
		}

		// Start HTTP server
		startServer(cfg)
	},
}

func init() {
	ServeCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to serve on")
	ServeCmd.Flags().BoolVarP(&serveWatch, "watch", "w", true, "Watch for file changes and rebuild")
	ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Verbose output")
	ServeCmd.Flags().BoolVar(&serveDrafts, "drafts", false, "Include draft posts in the output")
	ServeCmd.Flags().BoolVar(&serveClean, "clean", true, "Clean the output directory before rebuilding")
}

// buildSite builds the entire site
func buildSite(cfg *config.Config, buildDrafts bool) error {
	// Parse posts
	posts, err := parser.ParsePosts(cfg.ContentDir)
	if err != nil {
		return fmt.Errorf("failed to parse posts: %w", err)
	}

	// Generate site
	outputDir := cfg.PublishDir
	if err := generator.Generate(posts, cfg, outputDir, false, buildDrafts, serveClean); err != nil {
		return fmt.Errorf("failed to generate site: %w", err)
	}

	return nil
}

// startServer starts the HTTP development server
func startServer(cfg *config.Config) {
	addr := fmt.Sprintf(":%d", servePort)
	server := newDevServer(addr, cfg.PublishDir)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nShutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	fmt.Printf("Development server started at http://localhost%s\n", addr)
	fmt.Printf("Press Ctrl+C to stop\n")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func newDevServer(addr, outputDir string) *http.Server {
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

					if err := buildSite(cfg, serveDrafts); err != nil {
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
	paths := []string{cfg.ContentDir, cfg.StaticDir, "templates"}

	if cfg.Theme != "" {
		themesDir := cfg.ThemesDir
		if themesDir == "" {
			themesDir = "themes"
		}

		paths = append(paths,
			filepath.Join(themesDir, cfg.Theme, "layouts"),
			filepath.Join(themesDir, cfg.Theme, "assets"),
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
