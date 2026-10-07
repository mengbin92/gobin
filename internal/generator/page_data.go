package generator

import (
	"html/template"
	"strings"

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

	// Lang is the language key this page is rendered for ("" = the
	// default language or a monolingual site). Languages lists every
	// language of the site for building language switchers (nil on
	// monolingual sites). Both are v1.9.0 additions.
	Lang      string
	Languages []LanguageLink
}

// LanguageLink describes one site language for language-switcher UI.
type LanguageLink struct {
	// Lang is the language key ("" = the default language).
	Lang string
	// Name is the display name (the language's languageCode).
	Name string
	// URL is the root-relative home URL of the language ("/" for the
	// default language, "/<lang>/" otherwise, including any baseURL
	// path prefix).
	URL string
	// Active reports whether the current page is rendered in this
	// language.
	Active bool
}

// languageLinksFor builds the sorted language list for page data. The
// default language always comes first (its key "" sorts before every
// valid language key). It returns nil for monolingual sites so existing
// templates never observe the feature.
func languageLinksFor(cfg *config.Config) []LanguageLink {
	if cfg == nil || !cfg.IsMultilingual() {
		return nil
	}

	// On derived per-language configs the original (default-language)
	// identity is carried by RootBaseURL / RootLanguageCode; on the
	// default run those are empty and the config's own values apply.
	rootBaseURL := cfg.RootBaseURL
	rootLanguageCode := cfg.RootLanguageCode
	if rootBaseURL == "" && rootLanguageCode == "" && cfg.ActiveLanguage == "" {
		rootBaseURL = cfg.BaseURL
		rootLanguageCode = cfg.LanguageCode
	}

	links := make([]LanguageLink, 0, len(cfg.Languages)+1)
	links = append(links, LanguageLink{
		Lang:   "",
		Name:   languageDisplayName(rootLanguageCode, ""),
		URL:    siteURLPath(rootBaseURL, "/"),
		Active: cfg.ActiveLanguage == "",
	})
	for _, lang := range cfg.LanguageNames() {
		resolved := cfg.ResolveLanguage(lang)
		links = append(links, LanguageLink{
			Lang:   lang,
			Name:   languageDisplayName(resolved.LanguageCode, lang),
			URL:    siteURLPath(joinLangBaseURL(rootBaseURL, lang), "/"),
			Active: cfg.ActiveLanguage == lang,
		})
	}
	return links
}

// joinLangBaseURL mirrors config's unexported joinBaseURL: "" -> "/zh",
// "https://example.com/blog" -> "https://example.com/blog/zh".
func joinLangBaseURL(baseURL, lang string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "/" + lang
	}
	return base + "/" + lang
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
