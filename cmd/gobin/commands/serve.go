package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/spf13/cobra"
)

type devServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type serveOps struct {
	loadSiteInput             func() (*siteBuildInput, error)
	generateSite              func(*siteBuildInput, string, bool, bool, bool) error
	generateSiteWithResult    func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error)
	generateSiteWithOptionsFn func(*siteBuildInput, generator.GenerationOptions) (*generator.GenerationResult, error)
	startServer               func(io.Writer, *config.Config) error
	startServerWithRuntime    func(io.Writer, *config.Config, serveRuntime) error
	watchFiles                func(context.Context, *config.Config, serveRuntime)
}

type serveRuntime struct {
	stdout      io.Writer
	stderr      io.Writer
	buildDrafts bool
	cleanOutput bool
	incremental bool
	verbose     bool
	liveReload  *liveReloadBroker
}

type debounceCancelFunc func() bool
type debounceAfterFunc func(time.Duration, func()) debounceCancelFunc

var (
	servePort   int
	serveWatch  bool
	serveDrafts bool
	serveClean  bool
	serveReload bool
)

// ServeCmd is the serve command
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	Long: `Start a local development server with optional file watching and automatic rebuild.

The server watches for changes in your content and templates, automatically
rebuilds the site when files change, and can refresh open pages with LiveReload.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe(cmd.OutOrStdout(), serveDrafts, serveWatch)
	},
}

func init() {
	ServeCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to serve on")
	ServeCmd.Flags().BoolVarP(&serveWatch, "watch", "w", true, "Watch for file changes and rebuild")
	ServeCmd.Flags().BoolVar(&serveDrafts, "drafts", false, "Include draft posts in the output")
	ServeCmd.Flags().BoolVar(&serveClean, "clean", true, "Clean the output directory before rebuilding")
	ServeCmd.Flags().BoolVar(&serveReload, "live-reload", true, "Inject a development-only LiveReload client while watching")
}

func runServe(stdout io.Writer, buildDrafts bool, watch bool) error {
	return runServeWithOps(stdout, buildDrafts, watch, serveOps{
		loadSiteInput:             loadSiteBuildInput,
		generateSite:              generateSite,
		generateSiteWithResult:    generateSiteWithResult,
		generateSiteWithOptionsFn: generateSiteWithOptions,
		startServer:               startServer,
		startServerWithRuntime:    startServerWithRuntime,
		watchFiles:                watchFilesWithRuntime,
	})
}

func runServeWithOps(stdout io.Writer, buildDrafts bool, watch bool, ops serveOps) error {
	builder := newServeBuilderWithOptions(ops.loadSiteInput, ops.generateSite, ops.generateSiteWithResult, ops.generateSiteWithOptionsFn)
	fmt.Fprintln(stdout, "Building site...")
	input, result, err := builder.initialBuildResult(buildDrafts, serveClean)
	if err != nil {
		return fmt.Errorf("build site: %w", err)
	}
	cfg := input.cfg
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()

	printStaticAssetStats(stdout, result)
	fmt.Fprintln(stdout, "Site built successfully!")

	var liveReload *liveReloadBroker
	if watch && serveReload {
		liveReload = newLiveReloadBroker()
	}
	runtime := serveRuntime{
		stdout:     stdout,
		stderr:     os.Stderr,
		liveReload: liveReload,
	}

	if watch {
		runtime.buildDrafts = buildDrafts
		// Watcher rebuilds always run incremental: serveClean only governs
		// the initial build. Wiping the output dir on every file save would
		// throw away the manifest the watcher just primed.
		runtime.cleanOutput = false
		runtime.incremental = true
		runtime.verbose = verbose
		go ops.watchFiles(watchCtx, cfg, serveRuntime{
			stdout:      runtime.stdout,
			stderr:      runtime.stderr,
			buildDrafts: runtime.buildDrafts,
			cleanOutput: runtime.cleanOutput,
			incremental: runtime.incremental,
			verbose:     runtime.verbose,
			liveReload:  runtime.liveReload,
		})
	}

	if ops.startServerWithRuntime != nil {
		err = ops.startServerWithRuntime(stdout, cfg, runtime)
	} else {
		err = ops.startServer(stdout, cfg)
	}
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
	return newServeBuilder(loadInput, generate, nil).rebuild(serveRuntime{
		buildDrafts: buildDrafts,
		cleanOutput: cleanOutput,
	})
}

func buildSiteWithResultWithDeps(loadInput func() (*siteBuildInput, error), generate func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error), buildDrafts bool, cleanOutput bool) (*generator.GenerationResult, error) {
	return newServeBuilder(loadInput, nil, generate).rebuildResult(serveRuntime{
		buildDrafts: buildDrafts,
		cleanOutput: cleanOutput,
	})
}

func buildSiteWithRuntime(runtime serveRuntime) error {
	return buildSiteWithDeps(loadSiteBuildInput, generateSite, runtime.buildDrafts, runtime.cleanOutput)
}

func rebuildSiteAndReport(runtime serveRuntime) {
	rebuildSiteAndReportWithResultDeps(runtime, func(runtime serveRuntime) (*generator.GenerationResult, error) {
		return newServeBuilderWithOptions(loadSiteBuildInput, generateSite, generateSiteWithResult, generateSiteWithOptions).rebuildResult(runtime)
	})
}

func rebuildSiteAndReportWithDeps(runtime serveRuntime, rebuild func(serveRuntime) error) {
	if rebuild == nil {
		rebuildSiteAndReportWithResultDeps(runtime, nil)
		return
	}
	rebuildSiteAndReportWithResultDeps(runtime, func(runtime serveRuntime) (*generator.GenerationResult, error) {
		return nil, rebuild(runtime)
	})
}

func rebuildSiteAndReportWithResultDeps(runtime serveRuntime, rebuild func(serveRuntime) (*generator.GenerationResult, error)) {
	if rebuild == nil {
		fmt.Fprintln(runtime.stderr, "Error rebuilding site: rebuild function is nil")
		return
	}
	fmt.Fprintln(runtime.stdout, "\nRebuilding site...")
	start := time.Now()

	result, err := rebuild(runtime)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "Error rebuilding site: %v\n", err)
		return
	}

	elapsed := time.Since(start)
	printStaticAssetStats(runtime.stdout, result)
	fmt.Fprintf(runtime.stdout, "Site rebuilt successfully in %v\n", elapsed)
	if runtime.liveReload != nil {
		runtime.liveReload.notify()
	}
}
