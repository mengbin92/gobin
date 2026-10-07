package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// LanguageConfig holds per-language overrides for multilingual sites
// (v1.9.0). Every field is optional: unset fields inherit the top-level
// config value, except ContentDir / PageDir which default to a
// <contentDir>/<lang> / <pageDir>/<lang> subdirectory convention, and
// LanguageCode which defaults to the language key itself.
type LanguageConfig struct {
	// LanguageCode is the BCP-47-ish code emitted as <html lang=...> and
	// in feeds. Defaults to the language key.
	LanguageCode string `yaml:"languageCode"`
	// Title / Description override the site title and description for
	// this language.
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	// ContentDir / PageDir locate this language's posts and standalone
	// pages. Defaults: <contentDir>/<lang> and <pageDir>/<lang>.
	ContentDir string `yaml:"contentDir"`
	PageDir    string `yaml:"pageDir"`
	// Params is shallow-merged over the top-level params (language
	// entries win).
	Params map[string]interface{} `yaml:"params"`
	// Strings is merged over the top-level strings for the `T` template
	// function (language entries win).
	Strings map[string]string `yaml:"strings"`
}

// languageKeyPattern constrains language keys to a single safe URL path
// segment: they become the /<lang>/ output prefix.
var languageKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// IsMultilingual reports whether the site declares additional languages.
func (c *Config) IsMultilingual() bool {
	return c != nil && len(c.Languages) > 0
}

// LanguageNames returns the sorted list of declared language keys. The
// sort is load-bearing: map iteration order must never leak into build
// output.
func (c *Config) LanguageNames() []string {
	if c == nil || len(c.Languages) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Languages))
	for name := range c.Languages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveLanguage returns the effective configuration for a language:
// the declared overrides with defaults filled in. Defaults are resolved
// lazily here rather than baked in by Normalize so the incremental build
// env hash only reflects what the user actually wrote.
func (c *Config) ResolveLanguage(lang string) LanguageConfig {
	resolved := LanguageConfig{}
	if c != nil && c.Languages != nil {
		if lc, ok := c.Languages[lang]; ok && lc != nil {
			resolved = *lc
		}
	}
	// Nil-receiver safe: the convention defaults then resolve to the
	// bare language key, matching IsMultilingual / LanguageNames.
	var contentDir, pageDir string
	if c != nil {
		contentDir, pageDir = c.ContentDir, c.PageDir
	}
	if strings.TrimSpace(resolved.LanguageCode) == "" {
		resolved.LanguageCode = lang
	}
	if strings.TrimSpace(resolved.ContentDir) == "" {
		resolved.ContentDir = filepath.Join(contentDir, lang)
	}
	if strings.TrimSpace(resolved.PageDir) == "" {
		resolved.PageDir = filepath.Join(pageDir, lang)
	}
	return resolved
}

// DeriveForLanguage returns a shallow clone of the config scoped to one
// language: title/description/languageCode overrides applied, params and
// strings merged (language entries win), BaseURL extended with the
// /<lang> prefix so every joinURL consumer (canonical, feed, sitemap)
// produces language-prefixed absolute URLs automatically, and
// ActiveLanguage set. The original config is not modified.
func (c *Config) DeriveForLanguage(lang string) *Config {
	if c == nil {
		return nil
	}
	langCfg := c.ResolveLanguage(lang)

	derived := *c
	derived.Title = firstNonEmpty(langCfg.Title, c.Title)
	derived.Description = firstNonEmpty(langCfg.Description, c.Description)
	derived.LanguageCode = langCfg.LanguageCode
	derived.BaseURL = joinBaseURL(c.BaseURL, lang)
	derived.ContentDir = langCfg.ContentDir
	derived.PageDir = langCfg.PageDir
	derived.Params = mergeLangParams(c.Params, langCfg.Params)
	derived.Strings = mergeLangStrings(c.Strings, langCfg.Strings)
	derived.ActiveLanguage = lang
	derived.RootBaseURL = c.BaseURL
	derived.RootLanguageCode = c.LanguageCode
	return &derived
}

// joinBaseURL extends a baseURL with a language prefix: "" -> "/zh",
// "/" -> "/zh", "https://example.com/blog" -> "https://example.com/blog/zh".
func joinBaseURL(baseURL, lang string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "/" + lang
	}
	return base + "/" + lang
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func mergeLangParams(base, overlay map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	merged := make(map[string]interface{}, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged
}

func mergeLangStrings(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged
}

// validateLanguages checks the languages: section for safe keys and
// directory conflicts. It runs as part of ValidateInDir on the
// normalized config.
func validateLanguages(cfg *Config) error {
	if len(cfg.Languages) == 0 {
		return nil
	}

	// Language keys become root-level URL prefixes, so they must not
	// collide with the fixed root segments the generator already owns.
	reserved := map[string]string{
		normalizeConfigPath(cfg.PaginatePath): "paginatePath",
		"tags":                                "taxonomy",
		"categories":                          "taxonomy",
	}
	if seg := firstPathSegment(cfg.StaticDir); seg != "" {
		reserved[seg] = "staticDir"
	}
	for _, dir := range cfg.StaticDirs {
		if seg := firstPathSegment(dir); seg != "" {
			reserved[seg] = "staticDirs"
		}
	}
	if seg := firstPathSegment(cfg.PublishDir); seg != "" {
		reserved[seg] = "publishDir"
	}

	defaultContentDir := normalizeConfigPath(cfg.ContentDir)
	defaultPageDir := normalizeConfigPath(cfg.PageDir)
	publishDir := normalizeConfigPath(cfg.PublishDir)
	seenContentDirs := map[string]string{}
	seenPageDirs := map[string]string{}
	var contentDirList, pageDirList []langDir

	for _, lang := range cfg.LanguageNames() {
		if !languageKeyPattern.MatchString(lang) {
			return fmt.Errorf("invalid language key %q: must match %s", lang, languageKeyPattern.String())
		}
		if owner, clash := reserved[lang]; clash {
			return fmt.Errorf("invalid language key %q: conflicts with %s", lang, owner)
		}

		resolved := cfg.ResolveLanguage(lang)
		for _, check := range []struct {
			name     string
			dir      string
			fallback string
			seen     map[string]string
			list     *[]langDir
		}{
			{"contentDir", resolved.ContentDir, defaultContentDir, seenContentDirs, &contentDirList},
			{"pageDir", resolved.PageDir, defaultPageDir, seenPageDirs, &pageDirList},
		} {
			normalized := normalizeConfigPath(check.dir)
			if normalized == "" {
				return fmt.Errorf("invalid languages.%s.%s: must not be empty", lang, check.name)
			}
			if normalized == publishDir {
				return fmt.Errorf("invalid languages.%s.%s %q: must not overlap publishDir %q", lang, check.name, check.dir, cfg.PublishDir)
			}
			if normalized == check.fallback {
				return fmt.Errorf("invalid languages.%s.%s %q: must differ from the default %s %q", lang, check.name, check.dir, check.name, check.fallback)
			}
			// A language dir that CONTAINS the default dir would swallow
			// the default language's sources into the language's parse,
			// duplicating every post under /<lang>/.
			if configPathWithin(check.fallback, normalized) {
				return fmt.Errorf("invalid languages.%s.%s %q: must not contain the default %s %q", lang, check.name, check.dir, check.name, check.fallback)
			}
			if other, dup := check.seen[normalized]; dup {
				return fmt.Errorf("invalid languages.%s.%s %q: already used by language %q", lang, check.name, check.dir, other)
			}
			// Nesting between two languages' dirs double-counts the
			// nested language's sources in the outer language's parse.
			for _, prev := range *check.list {
				if configPathWithin(normalized, prev.dir) || configPathWithin(prev.dir, normalized) {
					return fmt.Errorf("invalid languages.%s.%s %q: must not nest inside or contain language %q's %s %q", lang, check.name, check.dir, prev.lang, check.name, prev.dir)
				}
			}
			check.seen[normalized] = lang
			*check.list = append(*check.list, langDir{lang: lang, dir: normalized})
		}
	}

	return nil
}

// langDir pairs a language key with its normalized directory for
// cross-language nesting checks.
type langDir struct {
	lang string
	dir  string
}

// configPathWithin reports whether child sits strictly inside parent,
// both already normalized by normalizeConfigPath (slash-separated).
func configPathWithin(child, parent string) bool {
	if child == "" || parent == "" || child == parent {
		return false
	}
	// A cleaned root spelling contains every other cleaned path.
	if parent == "." || parent == "/" {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}

func firstPathSegment(path string) string {
	normalized := normalizeConfigPath(path)
	if normalized == "" {
		return ""
	}
	segments := strings.SplitN(normalized, "/", 2)
	return segments[0]
}
