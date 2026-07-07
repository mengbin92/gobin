package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// chdirTemp creates a temp dir, chdirs into it, and restores the original
// working directory on cleanup. loadTemplates resolves _layouts/, _includes/,
// and templates/ relative to the process cwd, so tests must run inside the
// temp site root.
func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return tmp
}

// minimalDefaultTemplates writes the smallest legal templates/ tree so
// loadTemplates does not error with "no templates found".
func minimalDefaultTemplates(t *testing.T, root string) {
	base := `{{ define "base" }}{{ render .HeaderTemplate . }}{{ render .MainTemplate . }}{{ render .FooterTemplate . }}{{ end }}`
	single := `{{ define "singleMain" }}SINGLE:{{ .Title }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`
	pageTmpl := `{{ define "pageMain" }}PAGEMAIN:{{ .Title }}{{ end }}{{ define "pagePage" }}{{ template "base" . }}{{ end }}`
	list := `{{ define "listMain" }}LIST{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`
	notFound := `{{ define "notFoundMain" }}NF{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`
	header := `{{ define "header" }}H{{ end }}{{ define "headerNested" }}H{{ end }}`
	footer := `{{ define "footer" }}F{{ end }}{{ define "footerNested" }}F{{ end }}`

	mustWriteFile(t, filepath.Join(root, "templates", "_default", "base.html"), base)
	mustWriteFile(t, filepath.Join(root, "templates", "_default", "single.html"), single)
	mustWriteFile(t, filepath.Join(root, "templates", "_default", "page.html"), pageTmpl)
	mustWriteFile(t, filepath.Join(root, "templates", "_default", "list.html"), list)
	mustWriteFile(t, filepath.Join(root, "templates", "_default", "404.html"), notFound)
	mustWriteFile(t, filepath.Join(root, "templates", "partials", "header.html"), header)
	mustWriteFile(t, filepath.Join(root, "templates", "partials", "footer.html"), footer)
}

func TestLayoutDiscovery_RegistersBasenameTemplates(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	mustWriteFile(t, filepath.Join(tmp, "_layouts", "post.html"), `<article>LAYOUT-POST:{{ .Title }}</article>`)
	mustWriteFile(t, filepath.Join(tmp, "_includes", "header.html"), `<nav>INC-HEADER</nav>`)

	cfg := &config.Config{}
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	if tmpl.Lookup("post") == nil {
		t.Fatal(`expected _layouts/post.html registered as template "post"`)
	}
	if tmpl.Lookup("header") == nil {
		t.Fatal(`expected _includes/header.html registered as template "header"`)
	}
}

func TestLayoutDiscovery_BackwardCompatibleNoLayoutsDir(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	// No _layouts/ or _includes/ present.
	cfg := &config.Config{}
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	if tmpl.Lookup("singlePage") == nil {
		t.Fatal("expected default singlePage template still present")
	}
	if tmpl.Lookup("post") != nil {
		t.Fatal("did not expect a 'post' template without _layouts/")
	}
}

func TestPostLayout_SelectsLayoutTemplate(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	// Custom layout file. Using {{ .Content }} so we also verify injection.
	mustWriteFile(t, filepath.Join(tmp, "_layouts", "custom.html"),
		`<div class="custom-layout">{{ .Title }}|{{ .Content }}</div>`)

	post := &parser.Post{
		Title:       "Hello",
		Layout:      "custom",
		ContentHTML: "<p>body</p>",
	}
	post.URL = "/hello/"

	cfg := &config.Config{Title: "Test"}
	specs := buildPostPageSpecs([]*parser.Post{post}, cfg)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	got := specs[0].TemplateCandidates
	if len(got) == 0 || got[0] != "custom" {
		t.Fatalf("expected first candidate 'custom', got %v", got)
	}

	// Render end-to-end to confirm the layout template is actually selected.
	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	out, err := renderTemplateContent(tmpl, mustResolve(t, tmpl, specs[0].TemplateCandidates), specs[0].Data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !contains(s, "custom-layout") {
		t.Fatalf("expected layout marker in output, got: %s", s)
	}
	if !contains(s, "Hello|") {
		t.Fatalf("expected title in layout, got: %s", s)
	}
	if !contains(s, "<p>body</p>") {
		t.Fatalf("expected {{ .Content }} injected, got: %s", s)
	}
}

func TestPostLayout_DefaultPostFallsBackToSinglePage(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	// Post with default Layout ("post") but no _layouts/post.html.
	post := &parser.Post{
		Title:       "Default",
		Layout:      "post",
		ContentHTML: "<p>x</p>",
	}
	post.URL = "/default/"

	cfg := &config.Config{Title: "T"}
	specs := buildPostPageSpecs([]*parser.Post{post}, cfg)
	got := specs[0].TemplateCandidates
	// When layout is the default "post" and there is no matching template,
	// candidates must fall back to singlePage so rendering succeeds.
	if len(got) != 1 || got[0] != "singlePage" {
		t.Fatalf("expected fallback to [singlePage], got %v", got)
	}
}

func TestPageLayout_SelectsLayoutTemplate(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	mustWriteFile(t, filepath.Join(tmp, "_layouts", "landing.html"),
		`<section class="landing">{{ .Title }}::{{ .Content }}</section>`)

	page := &parser.Page{
		Title:       "About",
		Layout:      "landing",
		ContentHTML: "<h1>welcome</h1>",
	}
	page.URL = "/about/"

	cfg := &config.Config{Title: "T"}
	specs := buildStandalonePageSpecs([]*parser.Page{page}, cfg)
	got := specs[0].TemplateCandidates
	if len(got) == 0 || got[0] != "landing" {
		t.Fatalf("expected first candidate 'landing', got %v", got)
	}

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	out, err := renderTemplateContent(tmpl, mustResolve(t, tmpl, specs[0].TemplateCandidates), specs[0].Data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !contains(s, "landing") {
		t.Fatalf("expected landing layout marker, got: %s", s)
	}
	if !contains(s, "<h1>welcome</h1>") {
		t.Fatalf("expected content injected, got: %s", s)
	}
}

func TestIncludes_RenderedInLayout(t *testing.T) {
	tmp := chdirTemp(t)
	minimalDefaultTemplates(t, tmp)

	mustWriteFile(t, filepath.Join(tmp, "_includes", "nav.html"), `<nav class="inc-nav">NAV</nav>`)
	mustWriteFile(t, filepath.Join(tmp, "_layouts", "custom.html"),
		`{{ template "nav" . }}<main>{{ .Content }}</main>`)

	post := &parser.Post{
		Title:       "Inc",
		Layout:      "custom",
		ContentHTML: "<p>c</p>",
	}
	post.URL = "/inc/"

	cfg := &config.Config{Title: "T"}
	specs := buildPostPageSpecs([]*parser.Post{post}, cfg)

	tmpl, err := loadTemplates(cfg)
	if err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	out, err := renderTemplateContent(tmpl, mustResolve(t, tmpl, specs[0].TemplateCandidates), specs[0].Data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !contains(s, "inc-nav") {
		t.Fatalf("expected _includes/nav.html rendered via {{ template \"nav\" . }}, got: %s", s)
	}
}

func mustResolve(t *testing.T, tmpl renderer, candidates []string) string {
	t.Helper()
	name, err := resolveTemplateName(tmpl, candidates)
	if err != nil {
		t.Fatalf("resolveTemplateName %v: %v", candidates, err)
	}
	return name
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
