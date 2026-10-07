package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
	"github.com/mengbin92/gobin/internal/shortcode"
)

type siteBuildInput struct {
	cfg       *config.Config
	posts     []*parser.Post
	pages     []*parser.Page
	langPosts map[string][]*parser.Post
	langPages map[string][]*parser.Page
}

// loadSiteBuildInput loads the site using the default auto-concurrency for
// parsing (min(NumCPU, 4)). Callers that need to forward a user-specified
// --jobs value should use loadSiteBuildInputWithConcurrency instead.
func loadSiteBuildInput() (*siteBuildInput, error) {
	return loadSiteBuildInputWithConcurrency(0)
}

// loadSiteBuildInputWithConcurrency loads the site and parses content with
// the requested worker count. A concurrency of 0 (or negative) means auto.
// The worker count is forwarded to both the post and page parallel parsers
// so that --jobs controls parsing in lockstep with the page-render workers.
func loadSiteBuildInputWithConcurrency(concurrency int) (*siteBuildInput, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg = config.Normalize(cfg)

	renderOptions, err := renderOptionsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("load shortcodes: %w", err)
	}

	posts, pages, langPosts, langPages, err := parseSiteContent(cfg, renderOptions, concurrency)
	if err != nil {
		return nil, err
	}

	return &siteBuildInput{
		cfg:       cfg,
		posts:     posts,
		pages:     pages,
		langPosts: langPosts,
		langPages: langPages,
	}, nil
}

// parseSiteContent parses the default language's posts and pages, plus —
// for multilingual sites (v1.9.0) — every declared language's content.
// Language content directories nested inside the default contentDir /
// pageDir (the <dir>/<lang>/ convention) are excluded from the default
// language's parse so translated posts do not leak into the root site.
func parseSiteContent(cfg *config.Config, renderOptions parser.RenderOptions, concurrency int) (posts []*parser.Post, pages []*parser.Page, langPosts map[string][]*parser.Post, langPages map[string][]*parser.Page, err error) {
	var excludePostDirs, excludePageDirs []string
	if cfg.IsMultilingual() {
		for _, lang := range cfg.LanguageNames() {
			resolved := cfg.ResolveLanguage(lang)
			if isNestedDir(resolved.ContentDir, cfg.ContentDir) {
				excludePostDirs = append(excludePostDirs, resolved.ContentDir)
			}
			if isNestedDir(resolved.PageDir, cfg.PageDir) {
				excludePageDirs = append(excludePageDirs, resolved.PageDir)
			}
		}
	}

	posts, err = parser.ParsePostsWithOptionsConcurrentExclude(cfg.ContentDir, renderOptions, concurrency, excludePostDirs)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse posts: %w", err)
	}

	pages, err = parser.ParsePagesWithOptionsConcurrentExclude(cfg.PageDir, renderOptions, concurrency, excludePageDirs)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse pages: %w", err)
	}

	if !cfg.IsMultilingual() {
		return posts, pages, nil, nil, nil
	}

	langPosts = make(map[string][]*parser.Post, len(cfg.Languages))
	langPages = make(map[string][]*parser.Page, len(cfg.Languages))
	for _, lang := range cfg.LanguageNames() {
		resolved := cfg.ResolveLanguage(lang)
		langPosts[lang], err = parser.ParsePostsWithOptionsConcurrent(resolved.ContentDir, renderOptions, concurrency)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parse posts for language %q: %w", lang, err)
		}
		langPages[lang], err = parser.ParsePagesWithOptionsConcurrent(resolved.PageDir, renderOptions, concurrency)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parse pages for language %q: %w", lang, err)
		}
	}

	return posts, pages, langPosts, langPages, nil
}

// isNestedDir reports whether child sits strictly inside parent. Both
// paths are compared in absolute form (filepath.Abs is purely lexical,
// no I/O) so mixed spellings — a relative default contentDir with an
// absolute per-language override, or a "./"-prefixed dir — still nest
// correctly. Equality returns false: a language directory that equals
// the default directory is a config error, not a nesting case.
func isNestedDir(child, parent string) bool {
	childAbs, err := filepath.Abs(child)
	if err != nil {
		childAbs = filepath.Clean(child)
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		parentAbs = filepath.Clean(parent)
	}
	if childAbs == parentAbs {
		return false
	}
	return strings.HasPrefix(childAbs, parentAbs+string(filepath.Separator))
}

func renderOptionsFromConfig(cfg *config.Config) (parser.RenderOptions, error) {
	opts := parser.DefaultRenderOptions()
	if cfg != nil && cfg.Markup != nil && cfg.Markup.AllowUnsafeHTML != nil {
		opts.AllowUnsafeHTML = *cfg.Markup.AllowUnsafeHTML
	}

	registry, err := shortcode.LoadRegistry(cfg)
	if err != nil {
		return parser.RenderOptions{}, err
	}
	opts.Shortcodes = registry

	return opts, nil
}

func generateSite(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) error {
	_, err := generateSiteWithResult(input, outputDir, minify, buildDrafts, cleanOutput)
	return err
}

func generateSiteWithResult(input *siteBuildInput, outputDir string, minify bool, buildDrafts bool, cleanOutput bool) (*generator.GenerationResult, error) {
	return generateSiteWithOptions(input, generator.GenerationOptions{
		OutputDir:   outputDir,
		Minify:      minify,
		BuildDrafts: buildDrafts,
		CleanOutput: cleanOutput,
	})
}

func generateSiteWithOptions(input *siteBuildInput, opts generator.GenerationOptions) (*generator.GenerationResult, error) {
	if input == nil {
		return nil, fmt.Errorf("site build input is nil")
	}
	var result *generator.GenerationResult
	var err error
	if input.cfg.IsMultilingual() {
		result, err = generator.GenerateMultilingualWithOptions(
			generator.LanguageContent{Posts: input.posts, Pages: input.pages},
			languageContentMap(input.langPosts, input.langPages),
			input.cfg, opts)
	} else {
		result, err = generator.GenerateWithOptions(input.posts, input.pages, input.cfg, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("generate site: %w", err)
	}
	return result, nil
}

// languageContentMap zips the per-language post and page slices into
// generator.LanguageContent values keyed by language.
func languageContentMap(langPosts map[string][]*parser.Post, langPages map[string][]*parser.Page) map[string]generator.LanguageContent {
	content := make(map[string]generator.LanguageContent, len(langPosts))
	for lang, posts := range langPosts {
		entry := content[lang]
		entry.Posts = posts
		content[lang] = entry
	}
	for lang, pages := range langPages {
		entry := content[lang]
		entry.Pages = pages
		content[lang] = entry
	}
	return content
}
