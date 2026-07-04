package generator

import (
	"runtime"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type generationPlan struct {
	outputDir string
	pagePlan  pageRenderPlan
	artifacts artifactPipeline
	manifest  *BuildManifest
}

func prepareGenerationPlan(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string, minify bool, buildDrafts bool, incremental bool, concurrency int) (*generationPlan, error) {
	cfg = config.Normalize(cfg)
	if outputDir == "" {
		outputDir = cfg.PublishDir
	}

	content := prepareContentPlan(posts, standalonePages, cfg, buildDrafts)
	pagePlan := buildSitePagePlan(content.posts, content.standalonePages, cfg)

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	artifactSpecs := buildArtifactSpecs(content.posts, content.standalonePages, cfg, outputDir, pagePlan.tags, pagePlan.categories)
	plan := assembleGenerationPlan(outputDir, tmpl, pagePlan, artifactSpecs, minify, normalizeConcurrency(concurrency))

	manifest, err := buildManifestForRun(cfg, content.posts, content.standalonePages)
	if err != nil {
		return nil, err
	}
	plan.manifest = manifest

	if incremental {
		applyIncrementalSkips(plan, outputDir, manifest)
	}

	return plan, nil
}

func assembleGenerationPlan(outputDir string, tmpl renderer, pagePlan sitePagePlan, artifactSpecs []ArtifactSpec, minify bool, concurrency int) *generationPlan {
	return &generationPlan{
		outputDir: outputDir,
		pagePlan: pageRenderPlan{
			outputDir:   outputDir,
			templates:   tmpl,
			pages:       pagePlan.pages,
			concurrency: concurrency,
		},
		artifacts: artifactPipeline{
			specs: withMinifyArtifactEnabled(artifactSpecs, minify),
		},
	}
}

// autoConcurrencyCap bounds the worker count chosen for "auto" (--jobs 0).
// Page rendering writes many small files and is I/O-bound, so throughput peaks
// at a handful of workers and regresses past that as filesystem contention
// outweighs the gains (benchmarked: NumCPU=10 was slower than serial, while 4
// workers gave the best result). Explicit --jobs N is not capped, so power
// users with heavier, CPU-bound templates can opt into more.
const autoConcurrencyCap = 4

// normalizeConcurrency resolves a requested worker count into an effective one.
// A non-positive value means "auto", which maps to the number of CPUs capped at
// autoConcurrencyCap.
func normalizeConcurrency(concurrency int) int {
	if concurrency <= 0 {
		return min(runtime.NumCPU(), autoConcurrencyCap)
	}
	return concurrency
}

func (p *generationPlan) Execute(cleanOutput bool) error {
	return p.executeWith(cleanOutput, prepareOutputDir, func() error {
		return p.pagePlan.Execute()
	}, func() error {
		return p.artifacts.Execute()
	})
}

func (p *generationPlan) ExecuteResult(cleanOutput bool) (*GenerationResult, error) {
	result := &GenerationResult{}
	if err := prepareOutputDir(p.outputDir, cleanOutput); err != nil {
		return nil, err
	}
	pageStats, err := p.pagePlan.ExecuteResult()
	if err != nil {
		return nil, err
	}
	result.Pages = pageStats

	artifactResult, err := p.artifacts.ExecuteResult()
	if err != nil {
		return nil, err
	}
	if artifactResult != nil {
		result.StaticAssets = artifactResult.StaticAssets
		result.Artifacts = artifactResult.Artifacts
	}

	if err := writeBuildManifest(p.outputDir, p.manifest); err != nil {
		return nil, err
	}

	return result, nil
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
	outputDir   string
	templates   renderer
	pages       []PageSpec
	concurrency int
}

func (p pageRenderPlan) Execute() error {
	_, err := p.ExecuteResult()
	return err
}

func (p pageRenderPlan) ExecuteResult() (PageRenderStats, error) {
	stats, err := renderPageSpecsConcurrent(p.templates, p.outputDir, p.pages, p.concurrency)
	if err != nil {
		return PageRenderStats{}, err
	}
	return stats, nil
}
