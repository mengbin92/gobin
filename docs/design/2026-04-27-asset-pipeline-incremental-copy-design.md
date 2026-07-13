# Asset Pipeline Incremental Copy Design

## Goal

Upgrade Gobin's static asset pipeline so rebuilds skip copying unchanged static assets while preserving the current output paths and theme/site overlay behavior.

This is resource pipeline v1. It prepares the codebase for later fingerprinting and full incremental builds without changing public URLs or template contracts.

## Scope

### In Scope

- Add an explicit static asset copy planning step.
- Skip copying assets whose destination is already current.
- Preserve existing source collection and overlay semantics.
- Return copy statistics that can be asserted in tests.
- Keep `--clean=true` behavior unchanged.
- Keep `--clean=false` from deleting stale output files.
- Document the `--clean=false` static asset behavior.
- Update the optimization record.

### Out Of Scope

- Hash/fingerprint filenames.
- Rewriting template asset URLs.
- Deleting stale assets during non-clean builds.
- Full incremental page generation.
- Parallel resource copying.

## Current State

`copyStaticAssets` collects static files from theme assets and site assets, applies overlay rules, and copies every winning file to the output directory on every build. Site assets override theme assets for identical relative output paths. Missing site or theme asset directories are ignored.

This behavior is correct but wasteful during `serve` rebuilds and `build --clean=false`, where unchanged assets are rewritten even when the output file is already current.

## Design

### Asset Copy Planning

Introduce an internal copy plan:

```go
type staticAssetCopyAction string

const (
	staticAssetCopy staticAssetCopyAction = "copy"
	staticAssetSkip staticAssetCopyAction = "skip"
)

type staticAssetCopyPlan struct {
	Asset      staticAssetFile
	DestPath   string
	Action     staticAssetCopyAction
	Reason     string
}
```

Add:

```go
func planStaticAssetCopies(cfg *config.Config, outputDir string) ([]staticAssetCopyPlan, error)
```

The planner calls `collectStaticAssetFiles`, computes each destination path, stats source/destination, and decides whether to copy.

### Freshness Rules

The destination is current only when all of these are true:

- destination exists
- destination is a regular file
- source and destination sizes are equal
- source and destination permission modes are equal after masking to permission bits
- source modtime is not after destination modtime

If any check fails, the action is `copy`.

Reason values should be short and testable, for example:

- `missing`
- `not-regular`
- `size`
- `mode`
- `source-newer`
- `current`

### Execution

`copyStaticAssets` should call `executeStaticAssetCopyPlan`, copying only entries with `Action == staticAssetCopy`.

Add a small result type:

```go
type staticAssetCopyResult struct {
	Copied  int
	Skipped int
}
```

`copyStaticAssets` can continue returning only `error` to avoid changing artifact pipeline contracts. Tests can call the planner and executor directly.

### Non-Clean Builds

When `cleanOutput=false`, removed source assets should not be deleted from the output directory in this phase. Stale output cleanup is a separate policy and should remain owned by the existing clean-output behavior.

## Testing

Add tests for:

- unchanged destination plans as `skip`
- changed source content plans as `copy`
- changed destination mode plans as `copy`
- site assets still override theme assets in the plan
- executing a plan skips current files without changing destination modtime
- executing a plan copies changed files and updates content
- missing asset directories still produce no error

Run:

```bash
go test ./internal/generator -run 'TestPlanStaticAssetCopies|TestExecuteStaticAssetCopyPlan|TestCopyStaticAssets'
go test ./...
go vet ./...
```

## Documentation

Update README near `--clean=false`:

- unchanged static assets are skipped during non-clean rebuilds
- stale output files are not removed unless the build cleans the output directory

Update `docs/plans/2026-04-23-optimization-execution-plan.md` with a P2 resource pipeline v1 progress note.

## Acceptance Criteria

- Existing output paths remain unchanged.
- Theme/site overlay behavior remains unchanged.
- Re-running asset copy on current files skips writes.
- Changed source assets are copied.
- Full test suite and vet pass.
