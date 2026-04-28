package commands

import (
	"fmt"

	"github.com/mengbin92/gobin/internal/generator"
)

type serveBuilder struct {
	loadSiteInput          func() (*siteBuildInput, error)
	generateSite           func(*siteBuildInput, string, bool, bool, bool) error
	generateSiteWithResult func(*siteBuildInput, string, bool, bool, bool) (*generator.GenerationResult, error)
}

func (b serveBuilder) initialBuild(buildDrafts bool, cleanOutput bool) (*siteBuildInput, error) {
	input, _, err := b.initialBuildResult(buildDrafts, cleanOutput)
	return input, err
}

func (b serveBuilder) initialBuildResult(buildDrafts bool, cleanOutput bool) (*siteBuildInput, *generator.GenerationResult, error) {
	if b.loadSiteInput == nil {
		return nil, nil, fmt.Errorf("load site input function is nil")
	}
	if b.generateSite == nil && b.generateSiteWithResult == nil {
		return nil, nil, fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return nil, nil, err
	}

	result, err := b.runGenerate(input, input.cfg.PublishDir, false, buildDrafts, cleanOutput)
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
	if b.generateSite == nil && b.generateSiteWithResult == nil {
		return nil, fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return nil, err
	}
	return b.runGenerate(input, input.cfg.PublishDir, false, runtime.buildDrafts, runtime.cleanOutput)
}

func (b serveBuilder) runGenerate(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) (*generator.GenerationResult, error) {
	if b.generateSiteWithResult != nil {
		return b.generateSiteWithResult(input, outputDir, minify, buildDrafts, cleanOutput)
	}
	if err := b.generateSite(input, outputDir, minify, buildDrafts, cleanOutput); err != nil {
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
