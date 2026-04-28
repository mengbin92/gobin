package generator

import (
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

// GenerationResult summarizes observable work performed during generation.
type GenerationResult struct {
	StaticAssets AssetCopyStats
}

// AssetCopyStats summarizes static asset copy decisions.
type AssetCopyStats struct {
	Copied  int
	Skipped int
}

// Generate generates the static site.
func Generate(posts []*parser.Post, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	return GenerateWithPages(posts, nil, cfg, outputDir, minify, buildDrafts, cleanOutput)
}

func GenerateWithPages(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	_, err := GenerateWithPagesResult(posts, standalonePages, cfg, outputDir, minify, buildDrafts, cleanOutput)
	return err
}

func GenerateWithPagesResult(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) (*GenerationResult, error) {
	plan, err := prepareGenerationPlan(posts, standalonePages, cfg, outputDir, minify, buildDrafts)
	if err != nil {
		return nil, err
	}
	return plan.ExecuteResult(cleanOutput)
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
