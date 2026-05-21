package commands

import (
	"fmt"

	"github.com/mengbin92/gobin/internal/generator"
)

type serveBuilder struct {
	loadSiteInput             func() (*siteBuildInput, error)
	generateSite              func(*siteBuildInput, string, bool, bool, bool) error
	generateSiteWithResult    func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error)
	generateSiteWithOptionsFn func(*siteBuildInput, generator.GenerationOptions) (*generator.GenerationResult, error)
}

func (b serveBuilder) initialBuild(buildDrafts bool, cleanOutput bool) (*siteBuildInput, error) {
	input, _, err := b.initialBuildResult(buildDrafts, cleanOutput)
	return input, err
}

func (b serveBuilder) initialBuildResult(buildDrafts bool, cleanOutput bool) (*siteBuildInput, *generator.GenerationResult, error) {
	if b.loadSiteInput == nil {
		return nil, nil, fmt.Errorf("load site input function is nil")
	}
	if b.generateSite == nil && b.generateSiteWithResult == nil && b.generateSiteWithOptionsFn == nil {
		return nil, nil, fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return nil, nil, err
	}

	// Initial build always runs the full pipeline; passing Incremental=false
	// is intentional so the manifest is primed and the output dir is clean
	// (when CleanOutput=true).
	result, err := b.runGenerate(input, generator.GenerationOptions{
		OutputDir:   input.cfg.PublishDir,
		BuildDrafts: buildDrafts,
		CleanOutput: cleanOutput,
	})
	if err != nil {
		return nil, nil, err
	}

	return input, result, nil
}

func (b serveBuilder) rebuild(runtime serveRuntime) error {
	_, err := b.rebuildResult(runtime)
	return err
}

func (b serveBuilder) rebuildResult(runtime serveRuntime) (*generator.GenerationResult, error) {
	if b.loadSiteInput == nil {
		return nil, fmt.Errorf("load site input function is nil")
	}
	if b.generateSite == nil && b.generateSiteWithResult == nil && b.generateSiteWithOptionsFn == nil {
		return nil, fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return nil, err
	}
	// Watcher-driven rebuilds prefer the incremental path so unchanged
	// content does not pay for re-rendering. Falls back to the legacy
	// pointer when only the older signatures are wired up (which keeps
	// existing tests with mocked generate functions working).
	opts := generator.GenerationOptions{
		OutputDir:   input.cfg.PublishDir,
		BuildDrafts: runtime.buildDrafts,
		CleanOutput: runtime.cleanOutput,
		Incremental: runtime.incremental && !runtime.cleanOutput,
	}
	return b.runGenerate(input, opts)
}

func (b serveBuilder) runGenerate(input *siteBuildInput, opts generator.GenerationOptions) (*generator.GenerationResult, error) {
	if b.generateSiteWithOptionsFn != nil {
		return b.generateSiteWithOptionsFn(input, opts)
	}
	if b.generateSiteWithResult != nil {
		return b.generateSiteWithResult(input, opts.OutputDir, opts.Minify, opts.BuildDrafts, opts.CleanOutput)
	}
	if err := b.generateSite(input, opts.OutputDir, opts.Minify, opts.BuildDrafts, opts.CleanOutput); err != nil {
		return nil, err
	}
	return nil, nil
}

func newServeBuilder(loadSiteInput func() (*siteBuildInput, error), generateSite func(*siteBuildInput, string, bool, bool, bool) error, generateSiteWithResult func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error)) serveBuilder {
	return serveBuilder{
		loadSiteInput:          loadSiteInput,
		generateSite:           generateSite,
		generateSiteWithResult: generateSiteWithResult,
	}
}

func newServeBuilderWithOptions(
	loadSiteInput func() (*siteBuildInput, error),
	generateSite func(*siteBuildInput, string, bool, bool, bool) error,
	generateSiteWithResult func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error),
	generateSiteWithOptionsFn func(*siteBuildInput, generator.GenerationOptions) (*generator.GenerationResult, error),
) serveBuilder {
	return serveBuilder{
		loadSiteInput:             loadSiteInput,
		generateSite:              generateSite,
		generateSiteWithResult:    generateSiteWithResult,
		generateSiteWithOptionsFn: generateSiteWithOptionsFn,
	}
}
