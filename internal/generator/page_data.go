package generator

import (
	"html/template"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type BasePageData struct {
	Site           *config.Config
	Title          string
	MetaTitle      string
	Description    string
	Canonical      string
	OpenGraphType  string
	HeaderTemplate string
	FooterTemplate string
	MainTemplate   string

	// Content holds the rendered body HTML for layout-style templates that
	// use {{ .Content }} as the insertion point (Jekyll {{ content }}
	// equivalent). It is populated for single post pages and standalone
	// pages; it is empty for list / 404 pages whose templates do not
	// reference it.
	Content template.HTML
}

type ListPageData struct {
	BasePageData
	Posts      []*parser.Post
	Pagination Pagination
}

type SinglePageData struct {
	BasePageData
	Post     *parser.Post
	PrevPost *parser.Post
	NextPost *parser.Post
}

type StandalonePageData struct {
	BasePageData
	Page *parser.Page
}

type NotFoundPageData struct {
	BasePageData
}
