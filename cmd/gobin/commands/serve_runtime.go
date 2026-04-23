package commands

import "fmt"

type serveBuilder struct {
	loadSiteInput func() (*siteBuildInput, error)
	generateSite  func(*siteBuildInput, string, bool, bool, bool) error
}

func (b serveBuilder) initialBuild(buildDrafts bool, cleanOutput bool) (*siteBuildInput, error) {
	if b.loadSiteInput == nil {
		return nil, fmt.Errorf("load site input function is nil")
	}
	if b.generateSite == nil {
		return nil, fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return nil, err
	}
	if err := b.generateSite(input, input.cfg.PublishDir, false, buildDrafts, cleanOutput); err != nil {
		return nil, err
	}

	return input, nil
}

func (b serveBuilder) rebuild(runtime serveRuntime) error {
	if b.loadSiteInput == nil {
		return fmt.Errorf("load site input function is nil")
	}
	if b.generateSite == nil {
		return fmt.Errorf("generate site function is nil")
	}

	input, err := b.loadSiteInput()
	if err != nil {
		return err
	}
	return b.generateSite(input, input.cfg.PublishDir, false, runtime.buildDrafts, runtime.cleanOutput)
}

func newServeBuilder(loadSiteInput func() (*siteBuildInput, error), generateSite func(*siteBuildInput, string, bool, bool, bool) error) serveBuilder {
	return serveBuilder{
		loadSiteInput: loadSiteInput,
		generateSite:  generateSite,
	}
}
