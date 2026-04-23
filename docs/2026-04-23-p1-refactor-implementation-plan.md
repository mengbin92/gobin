# P1 Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete P1 by refactoring the dev server workflow, deduplicating search-index generation, and separating parser front matter input models from normalized runtime models without changing external behavior.

**Architecture:** Keep behavior stable while making boundaries explicit. Split `serve` into builder/watcher/server components behind the existing command entrypoint, move search document assembly into a shared builder, and isolate parser YAML decoding into dedicated front matter structs that are converted into normalized `Post` and `Page` values.

**Tech Stack:** Go, Cobra, fsnotify, html/template, YAML v3, Go test

---

## File Structure

- Modify: `cmd/gobin/commands/serve.go`
- Modify: `cmd/gobin/commands/serve_test.go`
- Create: `cmd/gobin/commands/serve_runtime.go`
- Create: `cmd/gobin/commands/serve_watcher.go`
- Create: `cmd/gobin/commands/serve_server.go`
- Modify: `internal/generator/search.go`
- Modify: `internal/generator/generator_test.go`
- Modify: `internal/parser/parser.go`
- Modify: `internal/parser/parser_test.go`
- Modify: `docs/2026-04-23-optimization-execution-plan.md`

### Task 1: Refactor `serve` Into Builder/Watcher/Server Components

**Files:**
- Modify: `cmd/gobin/commands/serve.go`
- Create: `cmd/gobin/commands/serve_runtime.go`
- Create: `cmd/gobin/commands/serve_watcher.go`
- Create: `cmd/gobin/commands/serve_server.go`
- Test: `cmd/gobin/commands/serve_test.go`

- [ ] **Step 1: Write failing tests for the extracted component boundaries**

Add tests that call new builder/watcher/server helpers directly and verify:
- builder performs initial build and rebuild with forwarded runtime options
- watcher owns debounce + watch-loop logic
- server owns HTTP lifecycle and handler wiring

- [ ] **Step 2: Run targeted tests to verify they fail**

Run: `go test ./cmd/gobin/commands -run 'TestServeBuilder|TestServeWatcher|TestServeServer'`
Expected: FAIL because the new helpers/types do not exist yet.

- [ ] **Step 3: Write minimal implementation**

Implement:
- `serveBuilder` for initial build / rebuild
- `serveWatcher` for watcher setup, registration, debounce, event loop
- `serveServer` for HTTP server creation and graceful shutdown

Keep `ServeCmd` and `runServeWithOps` as orchestration-only code.

- [ ] **Step 4: Run targeted tests to verify they pass**

Run: `go test ./cmd/gobin/commands -run 'TestServeBuilder|TestServeWatcher|TestServeServer|TestRunServeWithOps'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/gobin/commands/serve.go cmd/gobin/commands/serve_runtime.go cmd/gobin/commands/serve_watcher.go cmd/gobin/commands/serve_server.go cmd/gobin/commands/serve_test.go
git commit -m "refactor: split serve workflow components"
```

### Task 2: Deduplicate Search Index Document Assembly

**Files:**
- Modify: `internal/generator/search.go`
- Test: `internal/generator/generator_test.go`

- [ ] **Step 1: Write failing tests for shared search document building**

Add tests that verify:
- the same post metadata is used for full and minimal search docs
- only the full variant includes normalized content
- category/author/date formatting remain unchanged

- [ ] **Step 2: Run targeted tests to verify they fail**

Run: `go test ./internal/generator -run 'TestBuildSearchDocuments'`
Expected: FAIL because the shared builder does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Implement a shared document builder, e.g.:
- `buildSearchDocuments(posts, cfg, includeContent bool) []SearchDocument`
- `buildSearchDocument(post, cfg, includeContent bool) SearchDocument`

Keep output filenames and JSON shape unchanged.

- [ ] **Step 4: Run targeted tests to verify they pass**

Run: `go test ./internal/generator -run 'TestBuildSearchDocuments|TestGenerateSearch'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/generator/search.go internal/generator/generator_test.go
git commit -m "refactor: share search index document builder"
```

### Task 3: Split Parser Front Matter Inputs From Normalized Runtime Models

**Files:**
- Modify: `internal/parser/parser.go`
- Test: `internal/parser/parser_test.go`

- [ ] **Step 1: Write failing tests for front matter normalization boundaries**

Add tests that verify:
- YAML front matter decodes into dedicated input structs
- normalization produces the same `Post` / `Page` outputs as before
- slug/date/url fallback behavior remains unchanged

- [ ] **Step 2: Run targeted tests to verify they fail**

Run: `go test ./internal/parser -run 'TestNormalizePostFrontMatter|TestNormalizePageFrontMatter'`
Expected: FAIL because the dedicated front matter normalization helpers do not exist yet.

- [ ] **Step 3: Write minimal implementation**

Introduce separate YAML input structs and normalization helpers, such as:
- `postFrontMatter`
- `pageFrontMatter`
- `normalizePost(...)`
- `normalizePage(...)`

Keep `ParsePost` / `ParsePage` public signatures unchanged.

- [ ] **Step 4: Run targeted tests to verify they pass**

Run: `go test ./internal/parser -run 'TestNormalizePostFrontMatter|TestNormalizePageFrontMatter|TestParsePost|TestParsePage'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "refactor: split parser front matter normalization"
```

### Task 4: Verify, Update Docs, And Commit The Whole P1 Batch

**Files:**
- Modify: `docs/2026-04-23-optimization-execution-plan.md`

- [ ] **Step 1: Update the execution document**

Mark P1 items as completed and record what changed, verification evidence, and the final commit hash.

- [ ] **Step 2: Run full verification**

Run: `gofmt -w cmd/gobin/commands/serve.go cmd/gobin/commands/serve_runtime.go cmd/gobin/commands/serve_watcher.go cmd/gobin/commands/serve_server.go cmd/gobin/commands/serve_test.go internal/generator/search.go internal/generator/generator_test.go internal/parser/parser.go internal/parser/parser_test.go`

Run: `go test ./...`

Expected: PASS

- [ ] **Step 3: Create the requested unified commit**

```bash
git add cmd/gobin/commands/serve.go cmd/gobin/commands/serve_runtime.go cmd/gobin/commands/serve_watcher.go cmd/gobin/commands/serve_server.go cmd/gobin/commands/serve_test.go internal/generator/search.go internal/generator/generator_test.go internal/parser/parser.go internal/parser/parser_test.go docs/2026-04-23-optimization-execution-plan.md
git commit -m "Complete P1 refactor pass"
```
