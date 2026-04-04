package generator

import (
	"sort"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// Pagination represents pagination information for templates.
type Pagination struct {
	Page        int
	TotalPages  int
	TotalPosts  int
	PrevPage    int
	NextPage    int
	IsFirstPage bool
	IsLastPage  bool
}

// Generate generates the static site.
func Generate(posts []*parser.Post, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	return GenerateWithPages(posts, nil, cfg, outputDir, minify, buildDrafts, cleanOutput)
}

func GenerateWithPages(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	visiblePosts := preparePosts(posts, cfg, buildDrafts)

	sort.Slice(visiblePosts, func(i, j int) bool {
		return visiblePosts[i].Date.After(visiblePosts[j].Date)
	})

	if err := prepareOutputDir(outputDir, cleanOutput); err != nil {
		return err
	}

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		return err
	}

	pages, tags, categories := buildPageSpecs(visiblePosts, cfg)
	pages = append(pages, buildStandalonePageSpecs(standalonePages, cfg)...)
	if err := renderPageSpecs(tmpl, outputDir, pages); err != nil {
		return err
	}

	artifacts := buildArtifactSpecs(visiblePosts, cfg, outputDir, tags, categories)
	if err := executeArtifactSpecs(withMinifyArtifactEnabled(artifacts, minify)); err != nil {
		return err
	}

	return nil
}
func paginate(posts []*parser.Post, perPage int) [][]*parser.Post {
	if perPage <= 0 {
		perPage = 10
	}

	var pages [][]*parser.Post
	for i := 0; i < len(posts); i += perPage {
		end := i + perPage
		if end > len(posts) {
			end = len(posts)
		}
		pages = append(pages, posts[i:end])
	}
	return pages
}
