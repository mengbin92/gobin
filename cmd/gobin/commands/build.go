package commands

import (
	"fmt"
	"io"

	"github.com/mengbin92/gobin/internal/generator"
	"github.com/spf13/cobra"
)

var minify bool
var buildDrafts bool
var cleanOutput bool

// BuildCmd is the build command
var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the static site",
	Long: `Build the static site by parsing markdown posts and generating HTML pages.

This command will:
1. Read the configuration from config.yaml
2. Parse all markdown posts from the content directory
3. Generate HTML pages using templates
4. Copy static assets to the output directory

With --minify enabled, Gobin applies conservative HTML/CSS minification while
preserving JavaScript content to avoid unsafe rewrites.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(cmd.OutOrStdout(), minify, buildDrafts, cleanOutput)
	},
}

func runBuild(stdout io.Writer, minify bool, buildDrafts bool, cleanOutput bool) error {
	fmt.Fprintf(stdout, "Blog Static Site Generator v%s\n", Version)
	fmt.Fprintln(stdout, "===================================")

	input, err := loadSiteBuildInput()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Found %d posts\n", len(input.posts))

	result, err := generateSiteWithResult(input, input.cfg.PublishDir, minify, buildDrafts, cleanOutput)
	if err != nil {
		return err
	}
	printStaticAssetStats(stdout, result)

	if minify {
		fmt.Fprintf(stdout, "Site generated and minified successfully in '%s' directory\n", input.cfg.PublishDir)
	} else {
		fmt.Fprintf(stdout, "Site generated successfully in '%s' directory\n", input.cfg.PublishDir)
	}

	return nil
}

func printStaticAssetStats(stdout io.Writer, result *generator.GenerationResult) {
	if result == nil {
		return
	}
	fmt.Fprintf(stdout, "Pages: rendered %d, skipped %d\n", result.Pages.Rendered, result.Pages.Skipped)
	fmt.Fprintf(stdout, "Static assets: copied %d, skipped %d, deleted %d\n", result.StaticAssets.Copied, result.StaticAssets.Skipped, result.StaticAssets.Deleted)
}

func RunDefaultBuild(stdout io.Writer) error {
	return runBuild(stdout, false, false, true)
}

func init() {
	BuildCmd.Flags().BoolVar(&minify, "minify", false, "Apply conservative HTML/CSS minification while preserving JavaScript content")
	BuildCmd.Flags().BoolVar(&buildDrafts, "drafts", false, "Include draft posts in the output")
	BuildCmd.Flags().BoolVar(&cleanOutput, "clean", true, "Clean the output directory before generating")
}
