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
	Pages        PageRenderStats
	StaticAssets AssetCopyStats
}

// PageRenderStats summarizes rendered page work.
type PageRenderStats struct {
	Rendered int
	Skipped  int
}

// AssetCopyStats summarizes static asset copy decisions.
type AssetCopyStats struct {
	Copied  int
	Skipped int
	Deleted int
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

// DryRunReport summarizes a check-only pass over the site inputs. It is
// populated by DryRun for commands like `gobin check` that want to verify
// configuration and content without touching the output directory.
type DryRunReport struct {
	PostCount     int
	PageCount     int
	OutputCount   int
	CollidingURLs []DryRunCollision
}

// DryRunCollision describes a single permalink that multiple page outputs
// would write to under the current configuration.
type DryRunCollision struct {
	OutputPath string
	Sources    []string
}

// DryRun loads templates, computes the generation plan, and reports any
// permalink collisions without writing any files.
func DryRun(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, buildDrafts bool) (*DryRunReport, error) {
	plan, err := prepareGenerationPlan(posts, standalonePages, cfg, "", false, buildDrafts)
	if err != nil {
		return nil, err
	}

	report := &DryRunReport{
		PostCount:   len(posts),
		PageCount:   len(standalonePages),
		OutputCount: len(plan.pagePlan.pages),
	}

	sources := make(map[string][]string)
	order := make([]string, 0)
	for _, page := range plan.pagePlan.pages {
		if _, seen := sources[page.OutputPath]; !seen {
			order = append(order, page.OutputPath)
		}
		sources[page.OutputPath] = append(sources[page.OutputPath], page.Title)
	}
	for _, outputPath := range order {
		if len(sources[outputPath]) > 1 {
			report.CollidingURLs = append(report.CollidingURLs, DryRunCollision{
				OutputPath: outputPath,
				Sources:    sources[outputPath],
			})
		}
	}

	return report, nil
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
