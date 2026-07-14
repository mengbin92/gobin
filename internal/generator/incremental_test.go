package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func writeIncrementalGoldenTemplates(t *testing.T, siteDir string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ template "main" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "list.html"), `{{ define "main" }}list{{ end }}{{ define "listMain" }}{{ range .Posts }}{{ .Title }};{{ end }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "single.html"), `{{ define "main" }}single{{ end }}{{ define "singleMain" }}{{ .Post.Title }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`)
}

func incrementalCfg() *config.Config {
	return &config.Config{
		Title:        "Incremental Test",
		BaseURL:      "https://example.com",
		StaticDir:    "assets",
		ThemesDir:    "themes",
		Paginate:     5,
		PaginatePath: "page",
	}
}

func runIncrementalGenerate(t *testing.T, posts []*parser.Post, cfg *config.Config, outputDir string, incremental bool) *GenerationResult {
	t.Helper()
	result, err := GenerateWithOptions(posts, nil, cfg, GenerationOptions{
		OutputDir:   outputDir,
		Incremental: incremental,
		CleanOutput: false,
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions failed: %v", err)
	}
	return result
}

func TestGenerate_Incremental_FirstBuildRendersAll(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	post := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath}
	result := runIncrementalGenerate(t, []*parser.Post{post}, incrementalCfg(), outputDir, true)

	if result.Pages.Skipped != 0 {
		t.Fatalf("expected no skips on first build, got skipped=%d", result.Pages.Skipped)
	}
	if result.Pages.Rendered == 0 {
		t.Fatal("expected at least one page rendered on first build")
	}
}

func TestGenerate_Incremental_SecondBuildSkipsUnchangedPostPage(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	post := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath}

	// Prime the manifest with a non-incremental first build.
	_ = runIncrementalGenerate(t, []*parser.Post{post}, incrementalCfg(), outputDir, false)

	// Second build with --incremental should skip the unchanged post page.
	result := runIncrementalGenerate(t, []*parser.Post{post}, incrementalCfg(), outputDir, true)

	postOutput := filepath.Join(outputDir, "hello", "index.html")
	if _, err := os.Stat(postOutput); err != nil {
		t.Fatalf("post output should still exist: %v", err)
	}
	if result.Pages.Skipped == 0 {
		t.Fatal("expected at least one page to be skipped on unchanged second build")
	}
}

func TestGenerate_Incremental_ChangedPostInvalidatesItsOwnPage(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body v1")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	postV1 := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath, ContentHTML: "<p>v1</p>"}
	if _, err := GenerateWithOptions([]*parser.Post{postV1}, nil, incrementalCfg(), GenerationOptions{OutputDir: outputDir, CleanOutput: false}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Change the source file so the manifest hash mismatches.
	mustWriteFile(t, postPath, "post body v2")
	postV2 := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath, ContentHTML: "<p>v2</p>"}

	plan, err := prepareGenerationPlan([]*parser.Post{postV2}, nil, incrementalCfg(), outputDir, false, false, true, 1)
	if err != nil {
		t.Fatalf("prepareGenerationPlan: %v", err)
	}

	var found bool
	for _, page := range plan.pagePlan.pages {
		if page.OutputPath == "hello/index.html" {
			found = true
			if page.SkipReason != "" {
				t.Fatalf("expected changed post page to be re-rendered, got SkipReason=%q", page.SkipReason)
			}
		}
	}
	if !found {
		t.Fatal("expected hello/index.html to be in the page plan")
	}
}

func TestGenerate_Incremental_BuildEnvHashMismatchForcesFullRender(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	post := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath}
	cfgA := incrementalCfg()
	if _, err := GenerateWithOptions([]*parser.Post{post}, nil, cfgA, GenerationOptions{OutputDir: outputDir, CleanOutput: false}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Mutate something that contributes to build_env_hash.
	cfgB := incrementalCfg()
	cfgB.Title = "Changed Title"

	plan, err := prepareGenerationPlan([]*parser.Post{post}, nil, cfgB, outputDir, false, false, true, 1)
	if err != nil {
		t.Fatalf("prepareGenerationPlan: %v", err)
	}
	for _, page := range plan.pagePlan.pages {
		if strings.HasSuffix(page.OutputPath, "hello/index.html") && page.SkipReason != "" {
			t.Fatalf("expected build_env_hash change to invalidate skip on %s", page.OutputPath)
		}
	}
}

func TestGenerate_Incremental_JekyllTemplateChangesForceRender(t *testing.T) {
	tests := []struct {
		name       string
		changedDir string
		before     string
		after      string
		want       string
	}{
		{
			name:       "layout",
			changedDir: "_layouts",
			before:     `<article>layout-v1 {{ template "badge" . }} {{ .Content }}</article>`,
			after:      `<article>layout-v2 {{ template "badge" . }} {{ .Content }}</article>`,
			want:       "layout-v2",
		},
		{
			name:       "include",
			changedDir: "_includes",
			before:     `<span>include-v1</span>`,
			after:      `<span>include-v2</span>`,
			want:       "include-v2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			siteDir := filepath.Join(tmpDir, "site")
			outputDir := filepath.Join(siteDir, "public")
			postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
			mustWriteFile(t, postPath, "post body")
			writeIncrementalGoldenTemplates(t, siteDir)

			layoutPath := filepath.Join(siteDir, "_layouts", "custom.html")
			includePath := filepath.Join(siteDir, "_includes", "badge.html")
			mustWriteFile(t, layoutPath, `<article>layout-v1 {{ template "badge" . }} {{ .Content }}</article>`)
			mustWriteFile(t, includePath, `<span>include-v1</span>`)

			changedPath := filepath.Join(siteDir, tc.changedDir, map[string]string{
				"_layouts":  "custom.html",
				"_includes": "badge.html",
			}[tc.changedDir])
			mustWriteFile(t, changedPath, tc.before)

			oldWd, _ := os.Getwd()
			if err := os.Chdir(siteDir); err != nil {
				t.Fatalf("chdir: %v", err)
			}
			defer os.Chdir(oldWd)

			post := &parser.Post{
				Title:       "Hello",
				Slug:        "hello",
				URL:         "/hello/",
				Layout:      "custom",
				FilePath:    postPath,
				ContentHTML: "<p>body</p>",
			}
			cfg := incrementalCfg()
			_ = runIncrementalGenerate(t, []*parser.Post{post}, cfg, outputDir, false)

			mustWriteFile(t, changedPath, tc.after)
			result := runIncrementalGenerate(t, []*parser.Post{post}, cfg, outputDir, true)
			if result.Pages.Rendered == 0 {
				t.Fatalf("expected %s change to force page rendering", tc.changedDir)
			}

			output, err := os.ReadFile(filepath.Join(outputDir, "hello", "index.html"))
			if err != nil {
				t.Fatalf("read rendered post: %v", err)
			}
			if !strings.Contains(string(output), tc.want) {
				t.Fatalf("expected rendered post to contain %q, got %q", tc.want, output)
			}
		})
	}
}

func TestGenerate_Incremental_UnchangedSiteSkipsListAndAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "post body")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	post := &parser.Post{Title: "Hello", Slug: "hello", URL: "/hello/", FilePath: postPath, Tags: []string{"go"}, Categories: []string{"tech"}}
	cfg := incrementalCfg()
	cfg.EnableRobotsTXT = true

	first := runIncrementalGenerate(t, []*parser.Post{post}, cfg, outputDir, false)
	if first.Artifacts.Skipped != 0 {
		t.Fatalf("expected first build to skip no artifacts, got %d", first.Artifacts.Skipped)
	}
	if first.Artifacts.Ran == 0 {
		t.Fatal("expected first build to run at least one artifact")
	}

	second := runIncrementalGenerate(t, []*parser.Post{post}, cfg, outputDir, true)
	if second.Pages.Rendered != 0 {
		t.Fatalf("expected zero page renders on unchanged incremental build, got %d", second.Pages.Rendered)
	}
	if second.Artifacts.Skipped == 0 {
		t.Fatal("expected aggregate artifacts to be skipped on unchanged incremental build")
	}
	// The static assets artifact is intentionally NOT skipped because it has
	// its own incremental copy plan.
	if second.Artifacts.Ran == 0 {
		t.Fatal("expected at least the assets artifact to run on incremental build")
	}
}

func TestGenerate_Incremental_AddingPostInvalidatesAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postAPath := filepath.Join(siteDir, "_posts", "2026-01-01-a.md")
	mustWriteFile(t, postAPath, "a body")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	postA := &parser.Post{Title: "A", Slug: "a", URL: "/a/", FilePath: postAPath, Tags: []string{"go"}}
	if _, err := GenerateWithOptions([]*parser.Post{postA}, nil, incrementalCfg(), GenerationOptions{OutputDir: outputDir, CleanOutput: false}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	postBPath := filepath.Join(siteDir, "_posts", "2026-01-02-b.md")
	mustWriteFile(t, postBPath, "b body")
	postB := &parser.Post{Title: "B", Slug: "b", URL: "/b/", FilePath: postBPath, Tags: []string{"go"}}

	plan, err := prepareGenerationPlan([]*parser.Post{postA, postB}, nil, incrementalCfg(), outputDir, false, false, true, 1)
	if err != nil {
		t.Fatalf("prepareGenerationPlan: %v", err)
	}

	// New post B must NOT be skipped (no previous manifest entry).
	// The existing post A SHOULD still be skipped (unchanged source).
	// The list / taxonomy pages should NOT be skipped (post set changed).
	var sawB, sawList, sawTaxonomy bool
	for _, page := range plan.pagePlan.pages {
		switch page.OutputPath {
		case "b/index.html":
			sawB = true
			if page.SkipReason != "" {
				t.Fatalf("expected newly added post to be rendered, got SkipReason=%q", page.SkipReason)
			}
		case "index.html":
			sawList = true
			if page.SkipReason != "" {
				t.Fatalf("expected index list page to re-render after post added, got SkipReason=%q", page.SkipReason)
			}
		case "tags/go/index.html":
			sawTaxonomy = true
			if page.SkipReason != "" {
				t.Fatalf("expected tag page to re-render after post added, got SkipReason=%q", page.SkipReason)
			}
		}
	}
	if !sawB {
		t.Fatal("expected newly added post page in plan")
	}
	if !sawList {
		t.Fatal("expected index list page in plan")
	}
	if !sawTaxonomy {
		t.Fatal("expected tag page in plan")
	}

	// All aggregate artifacts must NOT be skipped because the post set changed.
	for _, artifact := range plan.artifacts.specs {
		if !isAggregateArtifact(artifact.Name) {
			continue
		}
		if artifact.SkipReason != "" {
			t.Fatalf("expected %s artifact to re-run after post added, got SkipReason=%q", artifact.Name, artifact.SkipReason)
		}
	}
}

func TestComputePostCategoryHashes_BodyOnlyEditOnlyAffectsFeedAndSearch(t *testing.T) {
	base := &parser.Post{
		Title:       "Hello",
		Slug:        "hello",
		URL:         "/hello/",
		Description: "A description",
		Summary:     "Summary",
		Tags:        []string{"go"},
		Categories:  []string{"tech"},
		Content:     "body v1",
		ContentHTML: "<p>body v1</p>",
	}
	edited := *base
	edited.Content = "body v2"
	edited.ContentHTML = "<p>body v2</p>"

	a := computePostCategoryHashes(base)
	b := computePostCategoryHashes(&edited)

	if a.List != b.List {
		t.Fatalf("body-only edit must not change ListHash, got %q vs %q", a.List, b.List)
	}
	if a.Sitemap != b.Sitemap {
		t.Fatalf("body-only edit must not change SitemapHash, got %q vs %q", a.Sitemap, b.Sitemap)
	}
	if a.Feed == b.Feed {
		t.Fatal("body edit must change FeedHash")
	}
	if a.Search == b.Search {
		t.Fatal("body edit must change SearchHash")
	}
}

func TestComputePostCategoryHashes_TitleEditAffectsAllCategories(t *testing.T) {
	base := &parser.Post{Title: "Hello", URL: "/hello/", Tags: []string{"go"}}
	edited := *base
	edited.Title = "Hello World"

	a := computePostCategoryHashes(base)
	b := computePostCategoryHashes(&edited)

	if a.List == b.List || a.Feed == b.Feed || a.Search == b.Search {
		t.Fatal("title edit must change List/Feed/Search hashes")
	}
}

func TestGenerate_Incremental_BodyOnlyEditSkipsListAndSitemap(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")
	postPath := filepath.Join(siteDir, "_posts", "2026-01-01-hello.md")
	mustWriteFile(t, postPath, "v1")
	writeIncrementalGoldenTemplates(t, siteDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	base := &parser.Post{
		Title:       "Hello",
		Slug:        "hello",
		URL:         "/hello/",
		Description: "Desc",
		Summary:     "Summary",
		Tags:        []string{"go"},
		FilePath:    postPath,
		Content:     "body v1",
		ContentHTML: "<p>body v1</p>",
	}
	cfg := incrementalCfg()
	if _, err := GenerateWithOptions([]*parser.Post{base}, nil, cfg, GenerationOptions{OutputDir: outputDir, CleanOutput: false}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Edit only the body (and source file) but keep title/date/tags/url etc.
	mustWriteFile(t, postPath, "v2")
	edited := *base
	edited.Content = "body v2"
	edited.ContentHTML = "<p>body v2</p>"

	plan, err := prepareGenerationPlan([]*parser.Post{&edited}, nil, cfg, outputDir, false, false, true, 1)
	if err != nil {
		t.Fatalf("prepareGenerationPlan: %v", err)
	}

	skipsByName := map[string]string{}
	for _, artifact := range plan.artifacts.specs {
		skipsByName[artifact.Name] = artifact.SkipReason
	}

	if skipsByName["sitemap"] == "" {
		t.Errorf("expected sitemap to be skipped on body-only edit, got SkipReason=%q", skipsByName["sitemap"])
	}
	if skipsByName["aliases"] == "" {
		t.Errorf("expected aliases to be skipped on body-only edit, got SkipReason=%q", skipsByName["aliases"])
	}
	if skipsByName["feed"] != "" {
		t.Errorf("expected feed to re-run on body edit, got SkipReason=%q", skipsByName["feed"])
	}
	if skipsByName["search"] != "" {
		t.Errorf("expected search to re-run on body edit, got SkipReason=%q", skipsByName["search"])
	}

	for _, page := range plan.pagePlan.pages {
		if page.OutputPath == "index.html" && page.SkipReason == "" {
			t.Errorf("expected list page to skip on body-only edit")
		}
		if page.OutputPath == "tags/go/index.html" && page.SkipReason == "" {
			t.Errorf("expected taxonomy page to skip on body-only edit")
		}
		if page.OutputPath == "hello/index.html" && page.SkipReason != "" {
			t.Errorf("expected modified post page to re-render, got SkipReason=%q", page.SkipReason)
		}
	}
}
