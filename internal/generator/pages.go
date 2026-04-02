package generator

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func buildPageSpecs(posts []*parser.Post, cfg *config.Config) ([]PageSpec, []string, []string) {
	pages := buildIndexPageSpecs(posts, cfg)
	pages = append(pages, buildNotFoundPageSpec(cfg))
	pages = append(pages, buildPostPageSpecs(posts, cfg)...)

	taxonomyPages, tags, categories := buildTaxonomyPageSpecs(posts, cfg)
	pages = append(pages, taxonomyPages...)

	return pages, tags, categories
}

func buildIndexPageSpecs(posts []*parser.Post, cfg *config.Config) []PageSpec {
	paginatedPosts := paginate(posts, cfg.Paginate)
	pages := make([]PageSpec, 0, len(paginatedPosts))

	for i, pagePosts := range paginatedPosts {
		pageNum := i + 1
		isFirstPage := pageNum == 1
		isLastPage := pageNum == len(paginatedPosts)

		paginationData := Pagination{
			Page:        pageNum,
			TotalPages:  len(paginatedPosts),
			TotalPosts:  len(posts),
			IsFirstPage: isFirstPage,
			IsLastPage:  isLastPage,
		}
		if !isFirstPage {
			paginationData.PrevPage = pageNum - 1
		}
		if !isLastPage {
			paginationData.NextPage = pageNum + 1
		}

		outputPath := "index.html"
		if !isFirstPage {
			outputPath = filepath.Join(cfg.PaginatePath, fmt.Sprintf("%d", pageNum), "index.html")
		}

		pages = append(pages, PageSpec{
			TemplateCandidates: []string{"listPage"},
			OutputPath:         outputPath,
			Data: ListPageData{
				BasePageData: BasePageData{
					Site:           cfg,
					Title:          cfg.Title,
					MetaTitle:      indexPageMetaTitle(pageNum),
					Description:    cfg.Description,
					Canonical:      canonicalForIndexPage(cfg, pageNum),
					OpenGraphType:  "website",
					HeaderTemplate: "header",
					FooterTemplate: "footer",
					MainTemplate:   "listMain",
				},
				Posts:      pagePosts,
				Pagination: paginationData,
			},
		})
	}

	if len(pages) == 0 {
		pages = append(pages, PageSpec{
			TemplateCandidates: []string{"listPage"},
			OutputPath:         "index.html",
			Data: ListPageData{
				BasePageData: BasePageData{
					Site:           cfg,
					Title:          cfg.Title,
					MetaTitle:      indexPageMetaTitle(1),
					Description:    cfg.Description,
					Canonical:      joinURL(cfg.BaseURL, "/"),
					OpenGraphType:  "website",
					HeaderTemplate: "header",
					FooterTemplate: "footer",
					MainTemplate:   "listMain",
				},
				Posts:      []*parser.Post{},
				Pagination: Pagination{Page: 1, TotalPages: 1, TotalPosts: 0, IsFirstPage: true, IsLastPage: true},
			},
		})
	}

	return pages
}

func indexPageMetaTitle(pageNum int) string {
	if pageNum <= 1 {
		return ""
	}

	return fmt.Sprintf("Page %d", pageNum)
}

func buildPostPageSpecs(posts []*parser.Post, cfg *config.Config) []PageSpec {
	pages := make([]PageSpec, 0, len(posts))

	for _, post := range posts {
		postPath := filepath.Join(strings.TrimPrefix(strings.TrimSuffix(post.URL, "/"), "/"), "index.html")
		pages = append(pages, PageSpec{
			TemplateCandidates: []string{"singlePage"},
			OutputPath:         postPath,
			Data: SinglePageData{
				BasePageData: BasePageData{
					Site:           cfg,
					Title:          post.Title,
					Description:    post.Description,
					Canonical:      joinURL(cfg.BaseURL, post.URL),
					OpenGraphType:  "article",
					HeaderTemplate: "headerNested",
					FooterTemplate: "footerNested",
					MainTemplate:   "singleMain",
				},
				Post: post,
			},
		})
	}

	return pages
}

func buildNotFoundPageSpec(cfg *config.Config) PageSpec {
	return PageSpec{
		TemplateCandidates: []string{"notFoundPage", "404Page"},
		OutputPath:         "404.html",
		Data: NotFoundPageData{
			BasePageData: BasePageData{
				Site:           cfg,
				Title:          "404 Not Found",
				Description:    "Page not found",
				Canonical:      joinURL(cfg.BaseURL, "/404.html"),
				OpenGraphType:  "website",
				HeaderTemplate: "header",
				FooterTemplate: "footer",
				MainTemplate:   "notFoundMain",
			},
		},
	}
}

func canonicalForIndexPage(cfg *config.Config, pageNum int) string {
	if pageNum <= 1 {
		return joinURL(cfg.BaseURL, "/")
	}

	return joinURL(cfg.BaseURL, fmt.Sprintf("/%s/%d/", strings.Trim(cfg.PaginatePath, "/"), pageNum))
}

func generateIndex(posts []*parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	return renderPageSpecs(tmpl, outputDir, buildIndexPageSpecs(posts, cfg))
}

func generatePost(post *parser.Post, cfg *config.Config, outputDir string, tmpl *template.Template) error {
	return renderPageSpecs(tmpl, outputDir, buildPostPageSpecs([]*parser.Post{post}, cfg))
}

func generateNotFoundPage(cfg *config.Config, outputDir string, tmpl *template.Template) error {
	return renderPageSpecs(tmpl, outputDir, []PageSpec{buildNotFoundPageSpec(cfg)})
}
