package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/spf13/cobra"
)

var checkIncludeDrafts bool
var checkAssets bool

// CheckCmd validates the site configuration and content without producing
// any output. Useful for CI gating and pre-deploy verification.
var CheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate config and content without building",
	Long: `Run the full build pipeline up to the point of rendering files and
report any problems. This catches:

- Config parse and validation errors
- Markdown / Front Matter parse errors
- Theme directory and template loading errors
- Permalink collisions (multiple pages writing to the same output path)
- (--assets only) Hash mismatches between fingerprinted asset filenames
  and on-disk content.

The command exits with a non-zero status when any error is detected.
Use --drafts to include draft posts in the collision check.
Use --assets to verify filename-level asset fingerprints after a build.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCheck(cmd.OutOrStdout(), cmd.ErrOrStderr(), checkIncludeDrafts, checkAssets)
	},
}

func init() {
	CheckCmd.Flags().BoolVar(&checkIncludeDrafts, "drafts", false, "Include draft posts when checking for permalink collisions")
	CheckCmd.Flags().BoolVar(&checkAssets, "assets", false, "Verify fingerprinted asset hashes against on-disk content (requires a prior build)")
}

func runCheck(stdout, stderr io.Writer, includeDrafts, assetsOnly bool) error {
	if assetsOnly {
		return runCheckAssets(stdout, stderr)
	}

	fmt.Fprintln(stdout, "Checking site...")
	log.Info("site check started", "include_drafts", includeDrafts)

	cfg, err := config.LoadDefault()
	if err != nil {
		log.Error("check failed: config", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] config: %v\n", err)
		return errors.New("check failed")
	}
	cfg = config.Normalize(cfg)
	fmt.Fprintln(stdout, "  [OK]   config loaded and validated")

	renderOpts, err := renderOptionsFromConfig(cfg)
	if err != nil {
		log.Error("check failed: shortcodes", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] shortcodes: %v\n", err)
		return errors.New("check failed")
	}
	posts, err := parser.ParsePostsWithOptionsConcurrent(cfg.ContentDir, renderOpts, 0)
	if err != nil {
		log.Error("check failed: posts", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] posts: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   parsed %d post(s) from %s\n", len(posts), cfg.ContentDir)

	pages, err := parser.ParsePagesWithOptionsConcurrent(cfg.PageDir, renderOpts, 0)
	if err != nil {
		log.Error("check failed: pages", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] pages: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   parsed %d page(s) from %s\n", len(pages), cfg.PageDir)

	report, err := generator.DryRun(posts, pages, cfg, includeDrafts)
	if err != nil {
		log.Error("check failed: templates / plan", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] templates / plan: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   templates loaded; %d page output(s) planned\n", report.OutputCount)

	if len(report.CollidingURLs) > 0 {
		for _, collision := range report.CollidingURLs {
			log.Error("check failed: permalink collision", "path", collision.OutputPath, "sources", strings.Join(collision.Sources, ","))
			fmt.Fprintf(stderr, "  [FAIL] permalink collision: %s\n", collision.OutputPath)
			for _, src := range collision.Sources {
				fmt.Fprintf(stderr, "          - %s\n", src)
			}
		}
		return fmt.Errorf("found %d permalink collision(s)", len(report.CollidingURLs))
	}
	fmt.Fprintln(stdout, "  [OK]   no permalink collisions")

	fmt.Fprintln(stdout, "Site check passed.")
	log.Info("site check passed")
	return nil
}

// runCheckAssets verifies the integrity of fingerprinted static assets in
// the publish directory. It reads the on-disk manifest and re-hashes every
// fingerprinted file, comparing the result against the hash embedded in its
// filename.
//
// Returns nil when all files are consistent, or fmt.Errorf listing every
// mismatch when any are found.
func runCheckAssets(stdout, stderr io.Writer) error {
	log.Info("asset check started")
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Error("asset check failed: config", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] config: %v\n", err)
		return errors.New("asset check failed")
	}
	cfg = config.Normalize(cfg)

	fp := generator.NewAssetFingerprinter(cfg)
	mismatches, verified, err := generator.VerifyAssetHashes(cfg.PublishDir, fp)
	if err != nil {
		log.Error("asset check failed: verify", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] verify: %v\n", err)
		return errors.New("asset check failed")
	}

	fmt.Fprintf(stdout, "  [OK]   verified %d fingerprinted asset(s) in %s\n", verified, cfg.PublishDir)
	if len(mismatches) == 0 {
		log.Info("asset check passed", "verified", verified)
		return nil
	}

	for _, m := range mismatches {
		log.Error("asset hash mismatch", "path", m.OutputPath, "expected", m.ExpectedHash, "actual", m.ActualHash)
		fmt.Fprintf(stderr, "  [FAIL] %s\n", m.String())
	}
	return fmt.Errorf("found %d asset hash mismatch(es)", len(mismatches))
}
