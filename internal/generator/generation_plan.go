package generator

import (
	"html/template"
	"sort"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type generationPlan struct {
	outputDir     string
	templates     *template.Template
	pageSpecs     []PageSpec
	artifactSpecs []ArtifactSpec
}

type pageBuildResult struct {
	pageSpecs  []PageSpec
	tags       []string
	categories []string
}

func prepareGenerationPlan(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool) (*generationPlan, error) {
	cfg = config.Normalize(cfg)
	if outputDir == "" {
		outputDir = cfg.PublishDir
	}

	visiblePosts := preparePosts(posts, cfg, buildDrafts)
	sortPostsByDateDesc(visiblePosts)

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	pageResult := buildSitePageSpecs(visiblePosts, standalonePages, cfg)
	artifactSpecs := buildArtifactSpecs(visiblePosts, cfg, outputDir, pageResult.tags, pageResult.categories)

	return &generationPlan{
		outputDir:     outputDir,
		templates:     tmpl,
		pageSpecs:     pageResult.pageSpecs,
		artifactSpecs: withMinifyArtifactEnabled(artifactSpecs, minify),
	}, nil
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
	if err := prepareOutputDir(p.outputDir, cleanOutput); err != nil {
		return err
	}
	if err := renderPageSpecs(p.templates, p.outputDir, p.pageSpecs); err != nil {
		return err
	}
	return executeArtifactSpecs(p.artifactSpecs)
}
