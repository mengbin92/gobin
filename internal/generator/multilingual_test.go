package generator

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func multilingualTestConfig() *config.Config {
	return &config.Config{
		Title:           "Multilingual Blog",
		Description:     "Default description.",
		LanguageCode:    "en",
		BaseURL:         "https://example.com",
		StaticDir:       "assets",
		ThemesDir:       "themes",
		Paginate:        10,
		PaginatePath:    "page",
		EnableRobotsTXT: true,
		Strings:         map[string]string{"readMore": "Read more"},
		Languages: map[string]*config.LanguageConfig{
			"zh": {
				LanguageCode: "zh-CN",
				Title:        "多语言博客",
				Strings:      map[string]string{"readMore": "阅读更多"},
			},
		},
	}
}

func multilingualTestPost(title, slug, key string, date time.Time) *parser.Post {
	return &parser.Post{
		Title:          title,
		Slug:           slug,
		Date:           date,
		TranslationKey: key,
		Content:        "body of " + slug,
		ContentHTML:    "<p>body of " + slug + "</p>",
		Summary:        "summary of " + slug,
		SummaryHTML:    "<p>summary of " + slug + "</p>",
		ReadingTime:    1,
	}
}

func TestLinkTranslations(t *testing.T) {
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	enPost := multilingualTestPost("Hello", "hello", "greeting", date)
	enSolo := multilingualTestPost("Solo", "solo", "", date)
	enDraft := multilingualTestPost("Draft", "draft", "draft-key", date)
	enDraft.Draft = true
	zhPost := multilingualTestPost("你好", "hello-zh", "greeting", date)

	cfg := multilingualTestConfig()
	runs := buildLanguageRuns(
		LanguageContent{Posts: []*parser.Post{enPost, enSolo, enDraft}},
		map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhPost}}},
		config.Normalize(cfg), GenerationOptions{OutputDir: "public"},
	)

	linkTranslations(runs, false)

	// Lang assignment covers every post, linked or not.
	if enPost.Lang != "" || enSolo.Lang != "" {
		t.Errorf("default-language posts Lang = %q/%q, want empty", enPost.Lang, enSolo.Lang)
	}
	if zhPost.Lang != "zh" {
		t.Errorf("zh post Lang = %q, want %q", zhPost.Lang, "zh")
	}

	// The linked pair sees each other with language-prefixed URLs.
	if len(enPost.Translations) != 1 {
		t.Fatalf("en post Translations = %v, want 1 entry", enPost.Translations)
	}
	link := enPost.Translations[0]
	if link.Lang != "zh" || link.Name != "zh-CN" || link.Title != "你好" || link.URL != "/zh/hello-zh/" {
		t.Errorf("en post translation link = %+v", link)
	}
	if len(zhPost.Translations) != 1 {
		t.Fatalf("zh post Translations = %v, want 1 entry", zhPost.Translations)
	}
	if got := zhPost.Translations[0]; got.Lang != "" || got.Name != "en" || got.Title != "Hello" || got.URL != "/hello/" {
		t.Errorf("zh post translation link = %+v", got)
	}

	// Posts without a key, and keys with a single visible member, stay unlinked.
	if enSolo.Translations != nil {
		t.Errorf("keyless post Translations = %v, want nil", enSolo.Translations)
	}
	if enDraft.Translations != nil {
		t.Errorf("draft-only group should stay unlinked: %v", enDraft.Translations)
	}
}

func TestLinkTranslations_DraftsIncludedWhenBuildingDrafts(t *testing.T) {
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	enDraft := multilingualTestPost("Draft", "draft", "shared", date)
	enDraft.Draft = true
	zhPost := multilingualTestPost("草稿", "draft-zh", "shared", date)

	cfg := multilingualTestConfig()
	runs := buildLanguageRuns(
		LanguageContent{Posts: []*parser.Post{enDraft}},
		map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhPost}}},
		config.Normalize(cfg), GenerationOptions{},
	)
	linkTranslations(runs, true)

	if len(enDraft.Translations) != 1 || len(zhPost.Translations) != 1 {
		t.Errorf("buildDrafts=true should link drafts: en=%v zh=%v", enDraft.Translations, zhPost.Translations)
	}
}

func TestLinkTranslations_SameLanguageKeyReuseStaysUnlinked(t *testing.T) {
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	// Two posts in the SAME language reusing a translationKey (e.g.
	// copy-pasted front matter) must not link to each other: links are
	// cross-language only.
	enA := multilingualTestPost("A", "a", "dup-key", date)
	enB := multilingualTestPost("B", "b", "dup-key", date)
	zhPost := multilingualTestPost("丙", "c-zh", "dup-key", date)

	cfg := multilingualTestConfig()
	runs := buildLanguageRuns(
		LanguageContent{Posts: []*parser.Post{enA, enB}},
		map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhPost}}},
		config.Normalize(cfg), GenerationOptions{},
	)
	linkTranslations(runs, false)

	for name, post := range map[string]*parser.Post{"enA": enA, "enB": enB} {
		if len(post.Translations) != 1 || post.Translations[0].Lang != "zh" {
			t.Errorf("%s should link only to zh, got %v", name, post.Translations)
		}
	}
	// The zh post links to BOTH default-language posts (the key is
	// ambiguous there, but every link is still cross-language).
	if len(zhPost.Translations) != 2 {
		t.Fatalf("zh Translations = %v, want links to both default posts", zhPost.Translations)
	}
	for _, link := range zhPost.Translations {
		if link.Lang != "" {
			t.Errorf("zh post should link only to the default language: %v", zhPost.Translations)
		}
	}
}

func TestLanguageLinksFor(t *testing.T) {
	if got := languageLinksFor(&config.Config{Title: "x"}); got != nil {
		t.Fatalf("monolingual languageLinksFor = %v, want nil", got)
	}

	cfg := config.Normalize(multilingualTestConfig())

	// Default-language view.
	links := languageLinksFor(cfg)
	if len(links) != 2 {
		t.Fatalf("languageLinksFor returned %d links, want 2", len(links))
	}
	if links[0].Lang != "" || !links[0].Active || links[0].URL != "/" || links[0].Name != "en" {
		t.Errorf("default link = %+v", links[0])
	}
	if links[1].Lang != "zh" || links[1].Active || links[1].URL != "/zh/" || links[1].Name != "zh-CN" {
		t.Errorf("zh link = %+v", links[1])
	}

	// Derived (zh) view: Active flips, root link still points at the
	// default language.
	derived := cfg.DeriveForLanguage("zh")
	zhLinks := languageLinksFor(derived)
	if len(zhLinks) != 2 {
		t.Fatalf("derived languageLinksFor returned %d links, want 2", len(zhLinks))
	}
	if zhLinks[0].Lang != "" || zhLinks[0].Active || zhLinks[0].URL != "/" || zhLinks[0].Name != "en" {
		t.Errorf("derived default link = %+v", zhLinks[0])
	}
	if zhLinks[1].Lang != "zh" || !zhLinks[1].Active || zhLinks[1].URL != "/zh/" {
		t.Errorf("derived zh link = %+v", zhLinks[1])
	}
}

func TestLanguageLinksFor_PathPrefixedBaseURL(t *testing.T) {
	cfg := config.Normalize(&config.Config{
		BaseURL:      "https://example.com/blog",
		LanguageCode: "en",
		Languages:    map[string]*config.LanguageConfig{"zh": {}},
	})

	links := languageLinksFor(cfg)
	if links[0].URL != "/blog/" {
		t.Errorf("default link URL = %q, want %q", links[0].URL, "/blog/")
	}
	if links[1].URL != "/blog/zh/" {
		t.Errorf("zh link URL = %q, want %q", links[1].URL, "/blog/zh/")
	}

	// The derived view must produce the same URLs (root identity is
	// carried on the derived config).
	derivedLinks := languageLinksFor(cfg.DeriveForLanguage("zh"))
	if derivedLinks[0].URL != "/blog/" || derivedLinks[1].URL != "/blog/zh/" {
		t.Errorf("derived links = %q, %q", derivedLinks[0].URL, derivedLinks[1].URL)
	}
}

func TestBuildArtifactSpecs_LanguageGating(t *testing.T) {
	cfg := config.Normalize(multilingualTestConfig())

	enabled := func(c *config.Config, name string) bool {
		for _, spec := range buildArtifactSpecs(nil, nil, c, "public", nil, nil) {
			if spec.Name == name {
				return spec.Enabled
			}
		}
		return false
	}

	if !enabled(cfg, "robots") || !enabled(cfg, "aliases") {
		t.Error("default language run should keep robots and aliases enabled")
	}

	derived := cfg.DeriveForLanguage("zh")
	if enabled(derived, "robots") {
		t.Error("non-default language run must not emit robots.txt")
	}
	if enabled(derived, "aliases") {
		t.Error("non-default language run must not emit alias pages")
	}
	for _, name := range []string{"feed", "sitemap", "search", "assets", "postprocess"} {
		if !enabled(derived, name) {
			t.Errorf("non-default language run should keep %q enabled", name)
		}
	}
}

func TestComputePostCategoryHashes_Translations(t *testing.T) {
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	post := multilingualTestPost("Hello", "hello", "greeting", date)
	post.URL = "/hello/"

	base := computePostCategoryHashes(post)

	linked := multilingualTestPost("Hello", "hello", "greeting", date)
	linked.URL = "/hello/"
	linked.Translations = []parser.TranslationLink{
		{Lang: "zh", Name: "zh-CN", Title: "你好", URL: "/zh/hello-zh/"},
	}
	withLink := computePostCategoryHashes(linked)

	if base.List == withLink.List {
		t.Error("ListHash should change when Translations are added")
	}

	// A URL change in the translation target must also flip the hash.
	moved := multilingualTestPost("Hello", "hello", "greeting", date)
	moved.URL = "/hello/"
	moved.Translations = []parser.TranslationLink{
		{Lang: "zh", Name: "zh-CN", Title: "你好", URL: "/zh/moved/"},
	}
	if withLink.List == computePostCategoryHashes(moved).List {
		t.Error("ListHash should change when a translation URL changes")
	}
}

func TestDryRunMultilingual(t *testing.T) {
	date := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	t.Run("same slug across languages is not a collision", func(t *testing.T) {
		cfg := config.Normalize(multilingualTestConfig())
		enPost := multilingualTestPost("Hello", "hello", "", date)
		zhPost := multilingualTestPost("你好", "hello", "", date)

		report, err := dryRunMultilingualInSite(t, LanguageContent{Posts: []*parser.Post{enPost}},
			map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhPost}}}, cfg)
		if err != nil {
			t.Fatalf("DryRunMultilingual failed: %v", err)
		}
		if len(report.CollidingURLs) != 0 {
			t.Errorf("collisions = %v, want none", report.CollidingURLs)
		}
	})

	t.Run("same slug within one language collides with language prefix", func(t *testing.T) {
		cfg := config.Normalize(multilingualTestConfig())
		zhA := multilingualTestPost("A", "dup", "", date)
		zhB := multilingualTestPost("B", "dup", "", date)

		report, err := dryRunMultilingualInSite(t, LanguageContent{},
			map[string]LanguageContent{"zh": {Posts: []*parser.Post{zhA, zhB}}}, cfg)
		if err != nil {
			t.Fatalf("DryRunMultilingual failed: %v", err)
		}
		if len(report.CollidingURLs) != 1 {
			t.Fatalf("collisions = %v, want 1", report.CollidingURLs)
		}
		if !strings.HasPrefix(report.CollidingURLs[0].OutputPath, "zh/") {
			t.Errorf("collision path = %q, want zh/ prefix", report.CollidingURLs[0].OutputPath)
		}
	})

	t.Run("default-language page under a language prefix collides across languages", func(t *testing.T) {
		cfg := config.Normalize(multilingualTestConfig())
		// A default-language page at /zh/ writes zh/index.html — the
		// same file the zh run's own home page writes.
		page := &parser.Page{Title: "Zh", URL: "/zh/"}

		report, err := dryRunMultilingualInSite(t, LanguageContent{Pages: []*parser.Page{page}},
			map[string]LanguageContent{"zh": {}}, cfg)
		if err != nil {
			t.Fatalf("DryRunMultilingual failed: %v", err)
		}
		if len(report.CollidingURLs) != 1 {
			t.Fatalf("collisions = %v, want 1 cross-language collision", report.CollidingURLs)
		}
		if report.CollidingURLs[0].OutputPath != "zh/index.html" {
			t.Errorf("collision path = %q, want zh/index.html", report.CollidingURLs[0].OutputPath)
		}
	})
}

// dryRunMultilingualInSite runs DryRunMultilingual inside a temp site
// dir carrying the repo templates (loadTemplates requires them on disk).
func dryRunMultilingualInSite(t *testing.T, defaultContent LanguageContent, langContent map[string]LanguageContent, cfg *config.Config) (*DryRunReport, error) {
	t.Helper()
	siteDir := t.TempDir()
	if err := createGoldenTestSite(siteDir); err != nil {
		t.Fatalf("create test site: %v", err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	return DryRunMultilingual(defaultContent, langContent, cfg, false)
}
