package generator

import (
	"sort"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type generationPlan struct {
	outputDir string
	pagePlan  pageRenderPlan
	artifacts artifactPipeline
}

type pageBuildResult struct {
	pageSpecs  []PageSpec
	tags       []string
	categories []string
}

type renderableContent struct {
	posts           []*parser.Post
	standalonePages []*parser.Page
	pageResult      pageBuildResult
}

func prepareGenerationPlan(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool) (*generationPlan, error) {
	cfg = config.Normalize(cfg)
	if outputDir == "" {
		outputDir = cfg.PublishDir
	}

	content := prepareRenderableContent(posts, standalonePages, cfg, buildDrafts)

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	artifactSpecs := buildArtifactSpecs(content.posts, cfg, outputDir, content.pageResult.tags, content.pageResult.categories)
	return assembleGenerationPlan(outputDir, tmpl, content.pageResult, artifactSpecs, minify), nil
}

func prepareRenderableContent(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, buildDrafts bool) renderableContent {
	visiblePosts := preparePosts(posts, cfg, buildDrafts)
	sortPostsByDateDesc(visiblePosts)
	pageResult := buildSitePageSpecs(visiblePosts, standalonePages, cfg)

	return renderableContent{
		posts:           visiblePosts,
		standalonePages: standalonePages,
		pageResult:      pageResult,
	}
}

func assembleGenerationPlan(outputDir string, tmpl renderer, pageResult pageBuildResult, artifactSpecs []ArtifactSpec, minify bool) *generationPlan {
	return &generationPlan{
		outputDir: outputDir,
		pagePlan: pageRenderPlan{
			outputDir: outputDir,
			templates: tmpl,
			pages:     pageResult.pageSpecs,
		},
		artifacts: artifactPipeline{
			specs: withMinifyArtifactEnabled(artifactSpecs, minify),
		},
	}
}

func buildSitePageSpecs(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config) pageBuildResult {
	pageSpecs, tags, categories := buildPageSpecs(posts, cfg)
	pageSpecs = append(pageSpecs, buildStandalonePageSpecs(standalonePages, cfg)...)

	return pageBuildResult{
		pageSpecs:  pageSpecs,
		tags:       tags,
		categories: categories,
	}
}

func sortPostsByDateDesc(posts []*parser.Post) {
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
}

func (p *generationPlan) Execute(cleanOutput bool) error {
	return p.executeWith(cleanOutput, prepareOutputDir, func() error {
		return p.pagePlan.Execute()
	}, func() error {
		return p.artifacts.Execute()
	})
}

func (p *generationPlan) executeWith(cleanOutput bool, prepare func(string, bool) error, renderPages func() error, runArtifacts func() error) error {
	if prepare == nil {
		return nil
	}
	if err := prepare(p.outputDir, cleanOutput); err != nil {
		return err
	}
	if renderPages == nil {
		return nil
	}
	if err := renderPages(); err != nil {
		return err
	}
	if runArtifacts == nil {
		return nil
	}
	return runArtifacts()
}

type pageRenderPlan struct {
	outputDir string
	templates renderer
	pages     []PageSpec
}

func (p pageRenderPlan) Execute() error {
	return renderPageSpecs(p.templates, p.outputDir, p.pages)
}
