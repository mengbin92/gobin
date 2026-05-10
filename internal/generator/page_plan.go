package generator

import (
	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type sitePagePlan struct {
	pages      []PageSpec
	tags       []string
	categories []string
}

func buildSitePagePlan(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config) sitePagePlan {
	pageSpecs, tags, categories := buildPageSpecs(posts, cfg)
	pageSpecs = append(pageSpecs, buildStandalonePageSpecs(standalonePages, cfg)...)

	return sitePagePlan{
		pages:      pageSpecs,
		tags:       tags,
		categories: categories,
	}
}
