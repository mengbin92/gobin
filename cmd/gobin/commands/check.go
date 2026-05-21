package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/spf13/cobra"
)

var checkIncludeDrafts bool

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

The command exits with a non-zero status when any error is detected.
Use --drafts to include draft posts in the collision check.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCheck(cmd.OutOrStdout(), cmd.ErrOrStderr(), checkIncludeDrafts)
	},
}

func init() {
	CheckCmd.Flags().BoolVar(&checkIncludeDrafts, "drafts", false, "Include draft posts when checking for permalink collisions")
}

func runCheck(stdout, stderr io.Writer, includeDrafts bool) error {
	fmt.Fprintln(stdout, "Checking site...")

	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "  [FAIL] config: %v\n", err)
		return errors.New("check failed")
	}
	cfg = config.Normalize(cfg)
	fmt.Fprintln(stdout, "  [OK]   config loaded and validated")

	renderOpts := renderOptionsFromConfig(cfg)
	posts, err := parser.ParsePostsWithOptions(cfg.ContentDir, renderOpts)
	if err != nil {
		fmt.Fprintf(stderr, "  [FAIL] posts: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   parsed %d post(s) from %s\n", len(posts), cfg.ContentDir)

	pages, err := parser.ParsePagesWithOptions(cfg.PageDir, renderOpts)
	if err != nil {
		fmt.Fprintf(stderr, "  [FAIL] pages: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   parsed %d page(s) from %s\n", len(pages), cfg.PageDir)

	report, err := generator.DryRun(posts, pages, cfg, includeDrafts)
	if err != nil {
		fmt.Fprintf(stderr, "  [FAIL] templates / plan: %v\n", err)
		return errors.New("check failed")
	}
	fmt.Fprintf(stdout, "  [OK]   templates loaded; %d page output(s) planned\n", report.OutputCount)

	if len(report.CollidingURLs) > 0 {
		for _, collision := range report.CollidingURLs {
			fmt.Fprintf(stderr, "  [FAIL] permalink collision: %s\n", collision.OutputPath)
			for _, src := range collision.Sources {
				fmt.Fprintf(stderr, "          - %s\n", src)
			}
		}
		return fmt.Errorf("found %d permalink collision(s)", len(report.CollidingURLs))
	}
	fmt.Fprintln(stdout, "  [OK]   no permalink collisions")

	fmt.Fprintln(stdout, "Site check passed.")
	return nil
}
