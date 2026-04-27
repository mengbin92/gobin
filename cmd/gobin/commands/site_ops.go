package commands

import (
	"fmt"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
)

type siteBuildInput struct {
	cfg   *config.Config
	posts []*parser.Post
	pages []*parser.Page
}

func loadSiteBuildInput() (*siteBuildInput, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg = config.Normalize(cfg)

	renderOptions := renderOptionsFromConfig(cfg)

	posts, err := parser.ParsePostsWithOptions(cfg.ContentDir, renderOptions)
	if err != nil {
		return nil, fmt.Errorf("parse posts: %w", err)
	}

	pages, err := parser.ParsePagesWithOptions(cfg.PageDir, renderOptions)
	if err != nil {
		return nil, fmt.Errorf("parse pages: %w", err)
	}

	return &siteBuildInput{
		cfg:   cfg,
		posts: posts,
		pages: pages,
	}, nil
}

func renderOptionsFromConfig(cfg *config.Config) parser.RenderOptions {
	opts := parser.DefaultRenderOptions()
	if cfg != nil && cfg.Markup != nil && cfg.Markup.AllowUnsafeHTML != nil {
		opts.AllowUnsafeHTML = *cfg.Markup.AllowUnsafeHTML
	}
	return opts
}

func generateSite(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	if input == nil {
		return fmt.Errorf("site build input is nil")
	}
	if err := generator.GenerateWithPages(input.posts, input.pages, input.cfg, outputDir, minify, buildDrafts, cleanOutput); err != nil {
		return fmt.Errorf("generate site: %w", err)
	}
	return nil
}
