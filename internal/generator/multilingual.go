package generator

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/mengbin92/gobin/internal/parser"
)

// LanguageContent holds the posts and standalone pages parsed for one
// language (v1.9.0).
type LanguageContent struct {
	Posts []*parser.Post
	Pages []*parser.Page
}

// languageRun is one language's pass through the generation pipeline:
// a (possibly derived) config plus the output directory subtree that
// language renders into. The default language run keeps the original
// config and output dir, so monolingual-equivalent behavior is
// preserved byte-for-byte.
type languageRun struct {
	Lang      string // "" = the default language
	Cfg       *config.Config
	Content   LanguageContent
	OutputDir string
}

// buildLanguageRuns assembles the per-language runs for a multilingual
// build: the default language first (it owns the site root and, with
// CleanOutput, wipes stale output from removed languages), then one run
// per declared language in sorted-key order.
func buildLanguageRuns(defaultContent LanguageContent, langContent map[string]LanguageContent, cfg *config.Config, opts GenerationOptions) []languageRun {
	cfg = config.Normalize(cfg)
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = cfg.PublishDir
	}

	runs := make([]languageRun, 0, len(cfg.Languages)+1)
	runs = append(runs, languageRun{
		Lang:      "",
		Cfg:       cfg,
		Content:   defaultContent,
		OutputDir: outputDir,
	})
	for _, lang := range cfg.LanguageNames() {
		runs = append(runs, languageRun{
			Lang:      lang,
			Cfg:       cfg.DeriveForLanguage(lang),
			Content:   langContent[lang],
			OutputDir: filepath.Join(outputDir, lang),
		})
	}
	return runs
}

// linkTranslations connects translations of the same post across
// languages. Every post gets its Lang set; posts sharing a non-empty
// TranslationKey with a visible post in another language get their
// Translations list populated (sorted by Lang, excluding themselves).
// It runs before any plan preparation so the per-language manifests —
// which fold Translations into the list hash — observe the final links.
func linkTranslations(runs []languageRun, buildDrafts bool) {
	type translationRef struct {
		post *parser.Post
		lang string
		name string
		url  string
	}

	groups := make(map[string][]translationRef)
	for _, run := range runs {
		name := languageDisplayName(run.Cfg.LanguageCode, run.Lang)
		for _, post := range run.Content.Posts {
			if post == nil {
				continue
			}
			post.Lang = run.Lang
			if post.TranslationKey == "" || !isVisiblePost(post, buildDrafts) {
				continue
			}
			groups[post.TranslationKey] = append(groups[post.TranslationKey], translationRef{
				post: post,
				lang: run.Lang,
				name: name,
				url:  siteURLPath(run.Cfg.BaseURL, buildPostURL(post, run.Cfg)),
			})
		}
		for _, page := range run.Content.Pages {
			if page != nil {
				page.Lang = run.Lang
			}
		}
	}

	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		sorted := append([]translationRef(nil), group...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].lang < sorted[j].lang })
		for _, ref := range group {
			links := make([]parser.TranslationLink, 0, len(group)-1)
			for _, other := range sorted {
				// Skip the post itself and same-language duplicates:
				// translationKey links translations ACROSS languages,
				// so two posts in one language sharing a key (e.g.
				// copy-pasted front matter) must not link to each
				// other.
				if other.post == ref.post || other.lang == ref.lang {
					continue
				}
				links = append(links, parser.TranslationLink{
					Lang:  other.lang,
					Name:  other.name,
					Title: other.post.Title,
					URL:   other.url,
				})
			}
			if len(links) > 0 {
				ref.post.Translations = links
			}
		}
	}
}

// languageDisplayName picks the human-facing label for a language: its
// languageCode when set, else the language key, else "default".
func languageDisplayName(languageCode, lang string) string {
	if languageCode != "" {
		return languageCode
	}
	if lang != "" {
		return lang
	}
	return "default"
}

// GenerateMultilingualWithOptions generates every language of a
// multilingual site. When the config declares no languages it forwards
// to the monolingual GenerateWithOptions unchanged.
//
// Architecture: each language runs the existing single-language
// pipeline with a derived config (BaseURL extended by /<lang>/) and a
// per-language output subtree (<outputDir>/<lang>/). Because every
// absolute URL flows through joinURL(cfg.BaseURL, ...) and every output
// path is relative to the run's output dir, feeds, sitemaps, search
// indexes, taxonomies, pagination, and manifests land under the
// language prefix with no changes to those code paths. Static assets
// and image variants are intentionally duplicated per language so
// assetURL stays internally consistent within each subtree.
func GenerateMultilingualWithOptions(defaultContent LanguageContent, langContent map[string]LanguageContent, cfg *config.Config, opts GenerationOptions) (*GenerationResult, error) {
	cfg = config.Normalize(cfg)
	if !cfg.IsMultilingual() {
		return GenerateWithOptions(defaultContent.Posts, defaultContent.Pages, cfg, opts)
	}

	logger := log.GetDefault().With("component", "generator")
	runs := buildLanguageRuns(defaultContent, langContent, cfg, opts)
	linkTranslations(runs, opts.BuildDrafts)

	total := &GenerationResult{}
	for _, run := range runs {
		logger.Debug("generating language",
			"lang", run.Lang,
			"posts", len(run.Content.Posts),
			"pages", len(run.Content.Pages),
			"output_dir", run.OutputDir,
		)
		plan, err := prepareGenerationPlan(run.Content.Posts, run.Content.Pages, run.Cfg, run.OutputDir, opts.Minify, opts.BuildDrafts, opts.Incremental, opts.Concurrency)
		if err != nil {
			return nil, fmt.Errorf("language %q: %w", languageDisplayName(run.Cfg.LanguageCode, run.Lang), err)
		}
		result, err := plan.ExecuteResult(opts.CleanOutput)
		if err != nil {
			return nil, fmt.Errorf("language %q: %w", languageDisplayName(run.Cfg.LanguageCode, run.Lang), err)
		}
		mergeGenerationResult(total, result)
	}
	return total, nil
}

// DryRunMultilingual is the multilingual analogue of DryRun. Output
// paths of non-default languages are prefixed with the language key, so
// identical slugs in different languages do not count as collisions —
// but collisions ACROSS languages (e.g. a default-language page whose
// URL falls under /zh/, overwriting the zh subtree's own index.html)
// are detected, which per-language isolation would miss.
func DryRunMultilingual(defaultContent LanguageContent, langContent map[string]LanguageContent, cfg *config.Config, buildDrafts bool) (*DryRunReport, error) {
	cfg = config.Normalize(cfg)
	if !cfg.IsMultilingual() {
		return DryRun(defaultContent.Posts, defaultContent.Pages, cfg, buildDrafts)
	}

	runs := buildLanguageRuns(defaultContent, langContent, cfg, GenerationOptions{})
	linkTranslations(runs, buildDrafts)

	report := &DryRunReport{}
	sources := make(map[string][]string)
	order := make([]string, 0)
	for _, run := range runs {
		plan, err := prepareGenerationPlan(run.Content.Posts, run.Content.Pages, run.Cfg, "", false, buildDrafts, false, 1)
		if err != nil {
			return nil, fmt.Errorf("language %q: %w", languageDisplayName(run.Cfg.LanguageCode, run.Lang), err)
		}
		report.PostCount += len(run.Content.Posts)
		report.PageCount += len(run.Content.Pages)
		report.OutputCount += len(plan.pagePlan.pages)

		prefix := ""
		if run.Lang != "" {
			prefix = run.Lang + "/"
		}
		for _, page := range plan.pagePlan.pages {
			outputPath := prefix + page.OutputPath
			if _, seen := sources[outputPath]; !seen {
				order = append(order, outputPath)
			}
			sources[outputPath] = append(sources[outputPath], page.Title)
		}
	}
	for _, outputPath := range order {
		if len(sources[outputPath]) > 1 {
			report.CollidingURLs = append(report.CollidingURLs, DryRunCollision{
				OutputPath: outputPath,
				Sources:    sources[outputPath],
			})
		}
	}
	return report, nil
}

// mergeGenerationResult accumulates per-language run statistics into a
// single build summary.
func mergeGenerationResult(total, result *GenerationResult) {
	if total == nil || result == nil {
		return
	}
	total.Pages.Rendered += result.Pages.Rendered
	total.Pages.Skipped += result.Pages.Skipped
	total.StaticAssets.Copied += result.StaticAssets.Copied
	total.StaticAssets.Skipped += result.StaticAssets.Skipped
	total.StaticAssets.Deleted += result.StaticAssets.Deleted
	total.Artifacts.Ran += result.Artifacts.Ran
	total.Artifacts.Skipped += result.Artifacts.Skipped
	total.Postprocess.HTMLFilesScanned += result.Postprocess.HTMLFilesScanned
	total.Postprocess.HTMLFilesChanged += result.Postprocess.HTMLFilesChanged
	total.Postprocess.ReferencesFound += result.Postprocess.ReferencesFound
	total.Postprocess.ReferencesRewritten += result.Postprocess.ReferencesRewritten
	total.Postprocess.ImageReferencesFound += result.Postprocess.ImageReferencesFound
	total.Postprocess.ImageReferencesRewritten += result.Postprocess.ImageReferencesRewritten
	total.Images.Sources += result.Images.Sources
	total.Images.Variants += result.Images.Variants
	total.Images.Skipped += result.Images.Skipped
}
