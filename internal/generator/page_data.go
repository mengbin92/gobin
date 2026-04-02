package generator

import (
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
}

type ListPageData struct {
	BasePageData
	Posts      []*parser.Post
	Pagination Pagination
}

type SinglePageData struct {
	BasePageData
	Post *parser.Post
}

type NotFoundPageData struct {
	BasePageData
}
