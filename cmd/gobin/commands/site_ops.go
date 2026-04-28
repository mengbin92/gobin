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
	_, err := generateSiteWithResult(input, outputDir, minify, buildDrafts, cleanOutput)
	return err
}

func generateSiteWithResult(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) (*generator.GenerationResult, error) {
	if input == nil {
		return nil, fmt.Errorf("site build input is nil")
	}
	result, err := generator.GenerateWithPagesResult(input.posts, input.pages, input.cfg, outputDir, minify, buildDrafts, cleanOutput)
	if err != nil {
		return nil, fmt.Errorf("generate site: %w", err)
	}
	return result, nil
}
