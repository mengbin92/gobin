# Markdown Safety And Serve Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Markdown unsafe HTML rendering configurable and add stronger serve lifecycle regression coverage.

**Architecture:** Keep parser defaults compatible, add option-aware parser entrypoints, and let command-layer site loading translate config into parser options. Strengthen `serve` with tests using existing injection seams rather than real ports or real fsnotify.

**Tech Stack:** Go, `testing`, Cobra command helpers, `goldmark`, YAML config.

---

## File Structure

- Modify `internal/config/config.go`: add `MarkupConfig` and `Config.Markup`.
- Modify `internal/config/config_test.go`: cover loading `markup.allowUnsafeHTML`.
- Modify `internal/parser/parser.go`: add `RenderOptions`, option-aware parse functions, and option-aware Markdown rendering.
- Modify `internal/parser/parser_test.go`: cover default raw HTML preservation and disabled unsafe HTML.
- Modify `cmd/gobin/commands/site_ops.go`: parse posts/pages with config-derived render options.
- Modify `cmd/gobin/commands/cli_test.go` or a new focused command test: cover build input honoring disabled unsafe HTML.
- Modify `cmd/gobin/commands/serve_test.go`: add lifecycle/recovery coverage.
- Modify `README.md`: document `markup.allowUnsafeHTML`.

## Task 1: Config Model For Markdown Safety

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config test**

Add to `internal/config/config_test.go`:

```go
func TestLoadConfig_MarkupAllowUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `title: "Markup Test"
baseURL: "https://example.com"
markup:
  allowUnsafeHTML: false
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Markup == nil || cfg.Markup.AllowUnsafeHTML == nil {
		t.Fatalf("Expected markup.allowUnsafeHTML to be loaded")
	}
	if *cfg.Markup.AllowUnsafeHTML {
		t.Fatal("Expected allowUnsafeHTML=false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/config -run TestLoadConfig_MarkupAllowUnsafeHTML
```

Expected: FAIL because `Config.Markup` does not exist.

- [ ] **Step 3: Add config structs**

In `internal/config/config.go`, add:

```go
Markup *MarkupConfig `yaml:"markup"`
```

and:

```go
type MarkupConfig struct {
	AllowUnsafeHTML *bool `yaml:"allowUnsafeHTML"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/config -run TestLoadConfig_MarkupAllowUnsafeHTML
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "Add markdown safety config"
```

## Task 2: Option-Aware Markdown Parsing

**Files:**
- Modify: `internal/parser/parser.go`
- Modify: `internal/parser/parser_test.go`

- [ ] **Step 1: Write failing parser tests**

Add tests:

```go
func TestParsePageWithOptions_DisablesUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	pagePath := filepath.Join(tmpDir, "unsafe.md")
	content := `---
title: "Unsafe"
---

<script>alert("x")</script>
<div class="hero">hello</div>`
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	page, err := ParsePageWithOptions(pagePath, tmpDir, RenderOptions{AllowUnsafeHTML: false})
	if err != nil {
		t.Fatalf("ParsePageWithOptions failed: %v", err)
	}
	if strings.Contains(page.ContentHTML, "<script>") || strings.Contains(page.ContentHTML, `<div class="hero">`) {
		t.Fatalf("Expected raw HTML to be disabled, got %s", page.ContentHTML)
	}
}

func TestParsePage_DefaultPreservesUnsafeHTML(t *testing.T) {
	tmpDir := t.TempDir()
	pagePath := filepath.Join(tmpDir, "unsafe.md")
	content := `---
title: "Unsafe"
---

<div class="hero">hello</div>`
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}

	page, err := ParsePage(pagePath, tmpDir)
	if err != nil {
		t.Fatalf("ParsePage failed: %v", err)
	}
	if !strings.Contains(page.ContentHTML, `<div class="hero">hello</div>`) {
		t.Fatalf("Expected default parser to preserve raw HTML, got %s", page.ContentHTML)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/parser -run 'TestParsePageWithOptions_DisablesUnsafeHTML|TestParsePage_DefaultPreservesUnsafeHTML'
```

Expected: FAIL because `RenderOptions` and `ParsePageWithOptions` do not exist.

- [ ] **Step 3: Add render options and option-aware APIs**

In `internal/parser/parser.go`, add:

```go
type RenderOptions struct {
	AllowUnsafeHTML bool
}

func DefaultRenderOptions() RenderOptions {
	return RenderOptions{AllowUnsafeHTML: true}
}
```

Change existing parse functions to delegate:

```go
func ParsePost(path string) (*Post, error) {
	return ParsePostWithOptions(path, DefaultRenderOptions())
}

func ParsePage(path string, baseDir string) (*Page, error) {
	return ParsePageWithOptions(path, baseDir, DefaultRenderOptions())
}
```

Add `ParsePostsWithOptions` and `ParsePagesWithOptions` mirroring the existing directory walkers.

Change rendering to:

```go
func renderMarkdownWithOptions(markdownContent string, opts RenderOptions) (string, error) {
	options := []goldmark.Option{}
	if opts.AllowUnsafeHTML {
		options = append(options, goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()))
	}
	md := goldmark.New(options...)
	...
}
```

Keep `renderMarkdown` as a compatibility wrapper around default options.

- [ ] **Step 4: Run parser tests**

Run:

```bash
go test ./internal/parser
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "Make markdown rendering options explicit"
```

## Task 3: Wire Config Into Build Input Loading

**Files:**
- Modify: `cmd/gobin/commands/site_ops.go`
- Modify: `cmd/gobin/commands/cli_test.go`

- [ ] **Step 1: Write failing build-input test**

Add a command-layer test that creates a temp site with `markup.allowUnsafeHTML: false`, a post containing raw HTML, and default templates. After `loadSiteBuildInput`, assert the parsed post `ContentHTML` does not contain the raw HTML tag.

Use `t.Chdir(tmpDir)` and the existing test helpers/patterns in `cmd/gobin/commands/cli_test.go`.

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./cmd/gobin/commands -run TestLoadSiteBuildInput_UsesMarkupRenderOptions
```

Expected: FAIL because `loadSiteBuildInput` still calls `parser.ParsePosts` / `parser.ParsePages`.

- [ ] **Step 3: Implement config-to-parser option mapping**

In `cmd/gobin/commands/site_ops.go`, add:

```go
func renderOptionsFromConfig(cfg *config.Config) parser.RenderOptions {
	opts := parser.DefaultRenderOptions()
	if cfg != nil && cfg.Markup != nil && cfg.Markup.AllowUnsafeHTML != nil {
		opts.AllowUnsafeHTML = *cfg.Markup.AllowUnsafeHTML
	}
	return opts
}
```

Use:

```go
renderOptions := renderOptionsFromConfig(cfg)
posts, err := parser.ParsePostsWithOptions(cfg.ContentDir, renderOptions)
pages, err := parser.ParsePagesWithOptions(cfg.PageDir, renderOptions)
```

- [ ] **Step 4: Run command tests**

Run:

```bash
go test ./cmd/gobin/commands -run 'TestLoadSiteBuildInput_UsesMarkupRenderOptions|TestRunBuild'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gobin/commands/site_ops.go cmd/gobin/commands/cli_test.go
git commit -m "Apply markdown render config during site loading"
```

## Task 4: Serve Lifecycle Recovery Tests

**Files:**
- Modify: `cmd/gobin/commands/serve_test.go`

- [ ] **Step 1: Write failing lifecycle recovery test**

Add a test that drives `runWatchLoop` with two rebuild-worthy events. Use a schedule function that immediately invokes the rebuild callback. Use a rebuild closure that fails first and succeeds second through `rebuildSiteAndReportWithDeps`.

Assert:

- stderr contains the first rebuild error.
- stdout contains a later successful rebuild message.
- rebuild was called twice.
- the loop exits only when the test cancels context or closes channels.

- [ ] **Step 2: Run test**

Run:

```bash
go test ./cmd/gobin/commands -run TestServeWatchLoop_RebuildFailureDoesNotStopLaterRebuild
```

Expected: PASS may already occur if existing behavior supports it. If it passes immediately, keep it as characterization coverage and do not force a production change.

- [ ] **Step 3: Add initial-build-before-server assertion if missing**

Add or extend a test around `runServeWithOps` to record operation order:

```go
order := []string{}
loadSiteInput: append "load"
generateSite: append "generate"
startServer: append "server"
```

Assert `load`, `generate`, `server`.

- [ ] **Step 4: Run serve tests**

Run:

```bash
go test ./cmd/gobin/commands -run 'TestServe'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gobin/commands/serve_test.go
git commit -m "Cover serve rebuild lifecycle recovery"
```

## Task 5: README Documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update config example**

Add:

```yaml
markup:
  allowUnsafeHTML: true
```

near the existing markup/highlight configuration.

- [ ] **Step 2: Add safety note**

Document:

- Default remains compatible and allows raw HTML.
- Set `markup.allowUnsafeHTML: false` for safer rendering when content is not fully trusted.

- [ ] **Step 3: Verify docs diff**

Run:

```bash
git diff -- README.md
```

Expected: README only changes the markup config documentation.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "Document markdown unsafe HTML setting"
```

## Task 6: Final Verification

**Files:**
- Verify all touched files.

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w internal/config/config.go internal/config/config_test.go internal/parser/parser.go internal/parser/parser_test.go cmd/gobin/commands/site_ops.go cmd/gobin/commands/cli_test.go cmd/gobin/commands/serve_test.go
```

- [ ] **Step 2: Run targeted tests**

Run:

```bash
go test ./internal/config ./internal/parser ./cmd/gobin/commands
```

Expected: PASS.

- [ ] **Step 3: Run full tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run vet**

Run:

```bash
go vet ./...
```

Expected: exit 0.

- [ ] **Step 5: Update optimization record if appropriate**

If the code work is completed, update `docs/2026-04-23-optimization-execution-plan.md` with a 2026-04-27 P2 progress note for Markdown safety and serve lifecycle coverage.

- [ ] **Step 6: Final commit**

If Task 5 or verification documentation changed after prior commits:

```bash
git add docs/2026-04-23-optimization-execution-plan.md
git commit -m "Record P2 optimization progress"
```

Final check:

```bash
git status --short
```
