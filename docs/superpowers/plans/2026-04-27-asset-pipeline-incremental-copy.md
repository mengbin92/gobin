# Asset Pipeline Incremental Copy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skip copying unchanged static assets while preserving current asset output paths and overlay semantics.

**Architecture:** Add a copy planning layer inside `internal/generator/assets.go` that classifies each collected asset as `copy` or `skip`. Keep `copyStaticAssets` as the artifact entrypoint, and expose the planner/executor only as package-internal helpers for focused tests.

**Tech Stack:** Go, `os.Stat`, `filepath`, package-level unit tests, existing Gobin generator pipeline.

---

## File Structure

- Modify `internal/generator/assets.go`: add static asset copy plan, freshness decision, executor, and result counts.
- Modify `internal/generator/generator_test.go`: add focused tests near existing asset tests.
- Modify `README.md`: document static asset skip behavior for non-clean builds.
- Modify `docs/2026-04-23-optimization-execution-plan.md`: record P2 resource pipeline v1 progress.

## Task 1: Plan Unchanged Assets As Skip

**Files:**
- Modify: `internal/generator/generator_test.go`
- Modify: `internal/generator/assets.go`

- [ ] **Step 1: Write the failing test**

Add near existing asset tests:

```go
func TestPlanStaticAssetCopies_SkipsCurrentDestination(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	mustWriteFile(t, filepath.Join(tmpDir, "assets", "css", "main.css"), "body{}")
	outputDir := filepath.Join(tmpDir, "public")
	destPath := filepath.Join(outputDir, "css", "main.css")
	mustWriteFile(t, destPath, "body{}")

	sourceInfo, err := os.Stat(filepath.Join(tmpDir, "assets", "css", "main.css"))
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	future := sourceInfo.ModTime().Add(time.Hour)
	if err := os.Chtimes(destPath, future, future); err != nil {
		t.Fatalf("set dest time: %v", err)
	}

	plans, err := planStaticAssetCopies(&config.Config{StaticDir: "assets"}, outputDir)
	if err != nil {
		t.Fatalf("planStaticAssetCopies failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("Expected 1 plan, got %d", len(plans))
	}
	if plans[0].Action != staticAssetSkip || plans[0].Reason != "current" {
		t.Fatalf("Expected current asset to be skipped, got %#v", plans[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/generator -run TestPlanStaticAssetCopies_SkipsCurrentDestination
```

Expected: FAIL because `planStaticAssetCopies` and action constants do not exist.

- [ ] **Step 3: Add minimal planner and freshness decision**

In `internal/generator/assets.go`, add internal types:

```go
type staticAssetCopyAction string

const (
	staticAssetCopy staticAssetCopyAction = "copy"
	staticAssetSkip staticAssetCopyAction = "skip"
)

type staticAssetCopyPlan struct {
	Asset    staticAssetFile
	DestPath string
	Action   staticAssetCopyAction
	Reason   string
}
```

Add:

```go
func planStaticAssetCopies(cfg *config.Config, outputDir string) ([]staticAssetCopyPlan, error) {
	assets, err := collectStaticAssetFiles(cfg)
	if err != nil {
		return nil, err
	}
	plans := make([]staticAssetCopyPlan, 0, len(assets))
	for _, asset := range assets {
		destPath := filepath.Join(outputDir, asset.OutputPath)
		action, reason, err := decideStaticAssetCopy(asset.SourcePath, destPath)
		if err != nil {
			return nil, err
		}
		plans = append(plans, staticAssetCopyPlan{
			Asset: asset, DestPath: destPath, Action: action, Reason: reason,
		})
	}
	return plans, nil
}
```

Add `decideStaticAssetCopy` using source/destination stat comparison.

- [ ] **Step 4: Run test to verify pass**

Run:

```bash
go test ./internal/generator -run TestPlanStaticAssetCopies_SkipsCurrentDestination
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/assets.go internal/generator/generator_test.go
git commit -m "Plan static asset copy decisions"
```

## Task 2: Plan Changed Assets As Copy

**Files:**
- Modify: `internal/generator/generator_test.go`
- Modify: `internal/generator/assets.go`

- [ ] **Step 1: Add failing tests for copy reasons**

Add table-style tests for:

- missing destination -> `copy`, `missing`
- size differs -> `copy`, `size`
- mode differs -> `copy`, `mode`
- source newer -> `copy`, `source-newer`

Use real files and `os.Chmod` / `os.Chtimes`.

- [ ] **Step 2: Run tests**

Run:

```bash
go test ./internal/generator -run TestPlanStaticAssetCopies_CopiesChangedAssets
```

Expected: FAIL until all reasons are implemented.

- [ ] **Step 3: Complete `decideStaticAssetCopy`**

Implement the exact order:

1. destination missing -> copy/missing
2. destination stat error -> return error
3. destination not regular -> copy/not-regular
4. source size differs -> copy/size
5. permission mode differs -> copy/mode
6. source modtime after destination -> copy/source-newer
7. otherwise skip/current

- [ ] **Step 4: Run targeted tests**

Run:

```bash
go test ./internal/generator -run 'TestPlanStaticAssetCopies'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/assets.go internal/generator/generator_test.go
git commit -m "Detect changed static assets"
```

## Task 3: Execute Copy Plan

**Files:**
- Modify: `internal/generator/assets.go`
- Modify: `internal/generator/generator_test.go`

- [ ] **Step 1: Write failing executor tests**

Add:

```go
func TestExecuteStaticAssetCopyPlan_SkipsCurrentFiles(t *testing.T) { ... }
func TestExecuteStaticAssetCopyPlan_CopiesChangedFiles(t *testing.T) { ... }
```

The skip test should record destination modtime before execution and assert it is unchanged after execution.

The copy test should change source content, execute the plan, and assert destination content updates.

- [ ] **Step 2: Run tests**

Run:

```bash
go test ./internal/generator -run TestExecuteStaticAssetCopyPlan
```

Expected: FAIL because executor does not exist.

- [ ] **Step 3: Add executor and result**

In `internal/generator/assets.go`, add:

```go
type staticAssetCopyResult struct {
	Copied  int
	Skipped int
}

func executeStaticAssetCopyPlan(plans []staticAssetCopyPlan) (staticAssetCopyResult, error) {
	var result staticAssetCopyResult
	for _, plan := range plans {
		if plan.Action == staticAssetSkip {
			result.Skipped++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(plan.DestPath), 0755); err != nil {
			return result, err
		}
		if err := copyFile(plan.Asset.SourcePath, plan.DestPath); err != nil {
			return result, err
		}
		result.Copied++
	}
	return result, nil
}
```

- [ ] **Step 4: Run executor tests**

Run:

```bash
go test ./internal/generator -run TestExecuteStaticAssetCopyPlan
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/assets.go internal/generator/generator_test.go
git commit -m "Execute static asset copy plans"
```

## Task 4: Route `copyStaticAssets` Through Planner

**Files:**
- Modify: `internal/generator/assets.go`
- Modify: `internal/generator/generator_test.go`

- [ ] **Step 1: Write failing integration-style asset test**

Add:

```go
func TestCopyStaticAssets_SkipsCurrentAssets(t *testing.T) { ... }
```

Call `copyStaticAssets` twice with unchanged assets and assert the second call does not change destination modtime.

- [ ] **Step 2: Run test**

Run:

```bash
go test ./internal/generator -run TestCopyStaticAssets_SkipsCurrentAssets
```

Expected: FAIL because `copyStaticAssets` still always rewrites.

- [ ] **Step 3: Update `copyStaticAssets`**

Replace direct copy loop with:

```go
plans, err := planStaticAssetCopies(cfg, outputDir)
if err != nil {
	return err
}
_, err = executeStaticAssetCopyPlan(plans)
return err
```

- [ ] **Step 4: Run asset tests**

Run:

```bash
go test ./internal/generator -run 'TestCopyStaticAssets|TestPlanStaticAssetCopies|TestExecuteStaticAssetCopyPlan|TestCollectStaticAssetFiles'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/assets.go internal/generator/generator_test.go
git commit -m "Skip unchanged static assets during copy"
```

## Task 5: Document Behavior And Progress

**Files:**
- Modify: `README.md`
- Modify: `docs/2026-04-23-optimization-execution-plan.md`

- [ ] **Step 1: Update README**

Near the `--clean=false` build option, add:

```markdown
When `--clean=false` is used, unchanged static assets are skipped during copy. Gobin does not remove stale output files in this mode; run a clean build to remove files that no longer exist in source assets.
```

- [ ] **Step 2: Update optimization record**

Add a 2026-04-27 P2 note:

- resource pipeline v1 completed
- unchanged assets are skipped
- stale deletion remains owned by clean builds
- fingerprinting remains future work

- [ ] **Step 3: Commit docs**

```bash
git add README.md docs/2026-04-23-optimization-execution-plan.md
git commit -m "Document static asset copy optimization"
```

## Task 6: Final Verification

**Files:**
- Verify all touched files.

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w internal/generator/assets.go internal/generator/generator_test.go
```

- [ ] **Step 2: Run targeted tests**

Run:

```bash
go test ./internal/generator -run 'TestPlanStaticAssetCopies|TestExecuteStaticAssetCopyPlan|TestCopyStaticAssets'
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

- [ ] **Step 5: Check worktree status**

Run:

```bash
git status --short
```

Expected: clean.
