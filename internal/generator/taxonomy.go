package generator

import (
	"html/template"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type TaxonomyTerm struct {
	Name  string
	URL   string
	Count int
}

type TaxonomyTermsPageData struct {
	BasePageData
	Kind    string
	BaseURL string
	Terms   []TaxonomyTerm
}

type TaxonomyPageData struct {
	BasePageData
	Kind     string
	Term     string
	Posts    []*parser.Post
	IndexURL string
}

func generateTaxonomyPages(posts []*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) ([]string, []string, error) {
	pages, tags, categories := buildTaxonomyPageSpecs(posts, cfg)
	if err := renderPageSpecs(tmpl, outputDir, pages); err != nil {
		return nil, nil, err
	}

	return tags, categories, nil
}

func collectTaxonomies(posts []*parser.Post) (map[string][]*parser.Post, map[string][]*parser.Post) {
	tagMap := make(map[string][]*parser.Post)
	categoryMap := make(map[string][]*parser.Post)

	for _, post := range posts {
		for _, tag := range post.Tags {
			tag = strings.ToLower(tag)
			tagMap[tag] = append(tagMap[tag], post)
		}
		for _, category := range post.Categories {
			category = strings.ToLower(category)
			categoryMap[category] = append(categoryMap[category], post)
		}
	}

	return tagMap, categoryMap
}

func sortedTaxonomyKeys(items map[string][]*parser.Post) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildTaxonomyPageSpecs(posts []*parser.Post, cfg *config.Config) ([]PageSpec, []string, []string) {
	tagMap, categoryMap := collectTaxonomies(posts)
	pages := make([]PageSpec, 0, len(tagMap)+len(categoryMap)+2)
	pages = append(pages, buildTaxonomyTermsPageSpec("tags", tagMap, cfg))
	pages = append(pages, buildTaxonomyEntryPageSpecs("tags", tagMap, cfg)...)
	pages = append(pages, buildTaxonomyTermsPageSpec("categories", categoryMap, cfg))
	pages = append(pages, buildTaxonomyEntryPageSpecs("categories", categoryMap, cfg)...)

	return pages, sortedTaxonomyKeys(tagMap), sortedTaxonomyKeys(categoryMap)
}

func buildTaxonomyTermsPageSpec(kind string, items map[string][]*parser.Post, cfg *config.Config) PageSpec {
	keys := sortedTaxonomyKeys(items)
	terms := make([]TaxonomyTerm, 0, len(keys))
	for _, key := range keys {
		terms = append(terms, TaxonomyTerm{
			Name:  key,
			URL:   "/" + kind + "/" + urlize(key) + "/",
			Count: len(items[key]),
		})
	}

	data := TaxonomyTermsPageData{
		BasePageData: BasePageData{
			Site:           cfg,
			Title:          taxonomyTitle(kind),
			Description:    taxonomyDescription(kind, ""),
			Canonical:      joinURL(cfg.BaseURL, "/"+kind+"/"),
			OpenGraphType:  "website",
			HeaderTemplate: "header",
			FooterTemplate: "footer",
			MainTemplate:   "taxonomyTermsMain",
		},
		Kind:    kind,
		BaseURL: "/" + kind + "/",
		Terms:   terms,
	}

	return PageSpec{
		TemplateCandidates: []string{"taxonomyTermsPage"},
		OutputPath:         filepath.Join(kind, "index.html"),
		Data:               data,
	}
}

func buildTaxonomyEntryPageSpecs(kind string, items map[string][]*parser.Post, cfg *config.Config) []PageSpec {
	pages := make([]PageSpec, 0, len(items))

	for _, key := range sortedTaxonomyKeys(items) {
		posts := append([]*parser.Post(nil), items[key]...)
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].Date.After(posts[j].Date)
		})

		data := TaxonomyPageData{
			BasePageData: BasePageData{
				Site:           cfg,
				Title:          taxonomyEntryTitle(kind, key),
				Description:    taxonomyDescription(kind, key),
				Canonical:      joinURL(cfg.BaseURL, "/"+kind+"/"+urlize(key)+"/"),
				OpenGraphType:  "website",
				HeaderTemplate: "header",
				FooterTemplate: "footer",
				MainTemplate:   "taxonomyMain",
			},
			Kind:     kind,
			Term:     key,
			Posts:    posts,
			IndexURL: "/" + kind + "/",
		}

		pages = append(pages, PageSpec{
			TemplateCandidates: []string{"taxonomyPage"},
			OutputPath:         filepath.Join(kind, urlize(key), "index.html"),
			Data:               data,
		})
	}

	return pages
}

func taxonomyTitle(kind string) string {
	if kind == "categories" {
		return "Categories"
	}
	return "Tags"
}

func taxonomyEntryTitle(kind, term string) string {
	if kind == "categories" {
		return "Category: " + term
	}
	return "Tag: " + term
}

func taxonomyDescription(kind, term string) string {
	if term == "" {
		return "Browse all " + kind
	}
	return "Posts in " + kind + " " + term
}
