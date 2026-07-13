# Markdown Safety And Serve Lifecycle Design

## Goal

Implement the next P2 optimization slice by making Markdown raw HTML rendering configurable and strengthening `serve` lifecycle regression coverage.

This slice deliberately avoids full incremental build work. It creates safer parser boundaries and better development-server tests so later resource pipeline and incremental-build work can proceed with less risk.

## Scope

### In Scope

- Add a configuration field for Markdown unsafe HTML rendering.
- Route build and serve parsing through that configuration.
- Keep existing parser APIs compatible for direct callers and existing tests.
- Add focused tests for config parsing and parser rendering behavior.
- Add `serve` lifecycle tests for initial build, watcher-triggered rebuilds, rebuild failure reporting, and recovery after failure.
- Update README configuration documentation.

### Out Of Scope

- Full incremental builds.
- Static asset fingerprinting.
- LiveReload injection.
- Reworking the HTTP server implementation.
- Rewriting the parser package around dependency injection.

## Current State

`internal/parser/renderMarkdown` currently always enables `goldmarkhtml.WithUnsafe()`. This preserves raw HTML in posts and pages, which is compatible with existing Jekyll-style content, but it gives users no explicit safety control.

`cmd/gobin/commands/serve.go` and related files already expose `serveOps`, `serveBuilder`, `serveWatcher`, and `serveServer` seams. Existing tests cover many helper behaviors, but they do not yet provide a more end-to-end lifecycle fixture that proves watch-triggered rebuilds recover after failures.

## Design

### Markdown Safety Configuration

Add a config structure:

```go
type MarkupConfig struct {
	AllowUnsafeHTML *bool `yaml:"allowUnsafeHTML"`
}
```

Attach it to `config.Config`:

```go
Markup *MarkupConfig `yaml:"markup"`
```

Use a pointer for `AllowUnsafeHTML` so the code can distinguish unset from explicitly false.

Default behavior remains compatible: unset means raw HTML is allowed, matching the current output. Users can opt into safer rendering with:

```yaml
markup:
  allowUnsafeHTML: false
```

Parser APIs gain options without breaking current callers:

```go
type RenderOptions struct {
	AllowUnsafeHTML bool
}

func DefaultRenderOptions() RenderOptions
func ParsePostWithOptions(path string, opts RenderOptions) (*Post, error)
func ParsePageWithOptions(path string, baseDir string, opts RenderOptions) (*Page, error)
func ParsePostsWithOptions(dir string, opts RenderOptions) ([]*Post, error)
func ParsePagesWithOptions(dir string, opts RenderOptions) ([]*Page, error)
```

Existing `ParsePost`, `ParsePage`, `ParsePosts`, and `ParsePages` call the option-aware versions with `DefaultRenderOptions()`.

`cmd/gobin/commands/site_ops.go` converts `cfg.Markup` to parser options before parsing posts and pages. That keeps config concerns out of the parser package and avoids an import cycle.

### Serve Lifecycle Coverage

Add tests that use temporary site inputs and injected `serveOps`/watcher functions rather than real ports or real filesystem notifications.

The lifecycle tests should prove:

- `runServeWithOps` runs initial build before server start.
- Watch-triggered rebuilds use the latest load result, not stale input.
- A rebuild failure is reported to stderr and does not stop later rebuild attempts.
- A later rebuild can succeed after a prior failure.

This can be covered by exercising `rebuildSiteAndReportWithDeps` and `runWatchLoop` together with a deterministic scheduler callback. The test should avoid sleeping where possible; if debounce behavior needs coverage, use the existing injectable scheduler seam.

## Error Handling

- Invalid YAML for `markup.allowUnsafeHTML` should fail through existing YAML decoding.
- Parser rendering errors remain wrapped by `ParsePost` / `ParsePage`.
- `serve` rebuild failures remain non-fatal to the watcher loop and are printed to stderr.

## Testing

Add tests for:

- Config loading with `markup.allowUnsafeHTML: false`.
- Parser default behavior preserving raw HTML.
- Option-aware parser behavior escaping or omitting raw HTML when unsafe HTML is false.
- Build input loading wiring config into parser options.
- Serve lifecycle recovery after a failed rebuild.

Run:

```bash
go test ./internal/config ./internal/parser ./cmd/gobin/commands
go test ./...
go vet ./...
```

## Documentation

Update README config example and configuration section with:

```yaml
markup:
  allowUnsafeHTML: true
```

Document that the default is currently compatible with existing raw HTML content, while setting it to `false` is safer for untrusted content.

## Acceptance Criteria

- Existing default builds keep preserving raw HTML.
- `markup.allowUnsafeHTML: false` changes build output so raw HTML is not rendered as active HTML.
- `serve` tests prove rebuild failures do not prevent later rebuilds.
- `go test ./...` and `go vet ./...` pass.
