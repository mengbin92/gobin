package generator

import (
	"fmt"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type ArtifactSpec struct {
	Name    string
	Enabled bool
	Run     func() error
}

type artifactPipeline struct {
	specs []ArtifactSpec
}

func buildArtifactSpecs(posts []*parser.Post, cfg *config.Config, outputDir string, tags, categories []string) []ArtifactSpec {
	hasAbsoluteBase := hasAbsoluteBaseURL(cfg.BaseURL)

	return []ArtifactSpec{
		{
			Name:    "feed",
			Enabled: outputEnabled(cfg, "feed", true) && hasAbsoluteBase,
			Run: func() error {
				return GenerateFeeds(posts, cfg, outputDir)
			},
		},
		{
			Name:    "sitemap",
			Enabled: outputEnabled(cfg, "sitemap", true) && hasAbsoluteBase,
			Run: func() error {
				return GenerateSitemap(posts, cfg, outputDir, tags, categories)
			},
		},
		{
			Name:    "search",
			Enabled: outputEnabled(cfg, "search", true),
			Run: func() error {
				return GenerateSearch(posts, cfg, outputDir)
			},
		},
		{
			Name:    "robots",
			Enabled: cfg != nil && cfg.EnableRobotsTXT && outputEnabled(cfg, "robots", true),
			Run: func() error {
				return generateRobotsTXT(cfg, outputDir)
			},
		},
		{
			Name:    "aliases",
			Enabled: true,
			Run: func() error {
				return generateAliasPages(posts, cfg, outputDir)
			},
		},
		{
			Name:    "assets",
			Enabled: true,
			Run: func() error {
				return copyStaticAssets(cfg, outputDir)
			},
		},
		{
			Name:    "minify",
			Enabled: false,
			Run: func() error {
				return minifyOutput(outputDir)
			},
		},
	}
}

func executeArtifactSpecs(specs []ArtifactSpec) error {
	return artifactPipeline{specs: specs}.Execute()
}

func (p artifactPipeline) Execute() error {
	for _, spec := range p.specs {
		if !spec.Enabled {
			continue
		}
		if err := spec.Run(); err != nil {
			if spec.Name == "" {
				return err
			}
			return fmt.Errorf("%s artifact: %w", spec.Name, err)
		}
	}

	return nil
}

func withMinifyArtifactEnabled(specs []ArtifactSpec, enabled bool) []ArtifactSpec {
	if !enabled {
		return specs
	}

	cloned := append([]ArtifactSpec(nil), specs...)
	for i := range cloned {
		if cloned[i].Name == "minify" {
			cloned[i].Enabled = true
		}
	}

	return cloned
}

func (p artifactPipeline) Names() []string {
	names := make([]string, 0, len(p.specs))
	for _, spec := range p.specs {
		names = append(names, spec.Name)
	}
	return names
}
