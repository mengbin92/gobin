package commands

import (
	"fmt"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/mengbin92/gobin/internal/shortcode"
)

type siteBuildInput struct {
	cfg   *config.Config
	posts []*parser.Post
	pages []*parser.Page
}

// loadSiteBuildInput loads the site using the default auto-concurrency for
// parsing (min(NumCPU, 4)). Callers that need to forward a user-specified
// --jobs value should use loadSiteBuildInputWithConcurrency instead.
func loadSiteBuildInput() (*siteBuildInput, error) {
	return loadSiteBuildInputWithConcurrency(0)
}

// loadSiteBuildInputWithConcurrency loads the site and parses content with
// the requested worker count. A concurrency of 0 (or negative) means auto.
// The worker count is forwarded to both the post and page parallel parsers
// so that --jobs controls parsing in lockstep with the page-render workers.
func loadSiteBuildInputWithConcurrency(concurrency int) (*siteBuildInput, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg = config.Normalize(cfg)

	renderOptions, err := renderOptionsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("load shortcodes: %w", err)
	}

	posts, err := parser.ParsePostsWithOptionsConcurrent(cfg.ContentDir, renderOptions, concurrency)
	if err != nil {
		return nil, fmt.Errorf("parse posts: %w", err)
	}

	pages, err := parser.ParsePagesWithOptionsConcurrent(cfg.PageDir, renderOptions, concurrency)
	if err != nil {
		return nil, fmt.Errorf("parse pages: %w", err)
	}

	return &siteBuildInput{
		cfg:   cfg,
		posts: posts,
		pages: pages,
	}, nil
}

func renderOptionsFromConfig(cfg *config.Config) (parser.RenderOptions, error) {
	opts := parser.DefaultRenderOptions()
	if cfg != nil && cfg.Markup != nil && cfg.Markup.AllowUnsafeHTML != nil {
		opts.AllowUnsafeHTML = *cfg.Markup.AllowUnsafeHTML
	}

	registry, err := shortcode.LoadRegistry(cfg)
	if err != nil {
		return parser.RenderOptions{}, err
	}
	opts.Shortcodes = registry

	return opts, nil
}

func generateSite(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	_, err := generateSiteWithResult(input, outputDir, minify, buildDrafts, cleanOutput)
	return err
}

func generateSiteWithResult(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) (*generator.GenerationResult, error) {
	return generateSiteWithOptions(input, generator.GenerationOptions{
		OutputDir:   outputDir,
		Minify:      minify,
		BuildDrafts: buildDrafts,
		CleanOutput: cleanOutput,
	})
}

func generateSiteWithOptions(input *siteBuildInput, opts generator.GenerationOptions) (*generator.GenerationResult, error) {
	if input == nil {
		return nil, fmt.Errorf("site build input is nil")
	}
	result, err := generator.GenerateWithOptions(input.posts, input.pages, input.cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("generate site: %w", err)
	}
	return result, nil
}
