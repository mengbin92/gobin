package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestNormalizeConcurrency(t *testing.T) {
	autoExpected := min(runtime.NumCPU(), autoConcurrencyCap)
	cases := []struct {
		in   int
		want int
	}{
		{-1, autoExpected},
		{0, autoExpected},
		{1, 1},
		{2, 2},
		{64, 64}, // explicit values are not capped
	}
	for _, c := range cases {
		if got := normalizeConcurrency(c.in); got != c.want {
			t.Errorf("normalizeConcurrency(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// parallelTestTemplate renders the page Data verbatim so each spec produces
// distinct, deterministic output we can compare byte-for-byte.
func parallelTestTemplate(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("page").Parse(`{{ define "page" }}content:{{ . }}{{ end }}`))
}

func parallelTestSpecs(n int) []PageSpec {
	pages := make([]PageSpec, 0, n)
	for i := 0; i < n; i++ {
		pages = append(pages, PageSpec{
			TemplateCandidates: []string{"page"},
			OutputPath:         filepath.Join(fmt.Sprintf("post-%03d", i), "index.html"),
			Data:               fmt.Sprintf("body-%03d", i),
		})
	}
	return pages
}

// snapshotDir returns a path->content map of every file under root.
func snapshotDir(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestRenderPageSpecsConcurrent_MatchesSerial verifies that parallel rendering
// produces the exact same file set, byte content, and stats as the serial path.
func TestRenderPageSpecsConcurrent_MatchesSerial(t *testing.T) {
	tmpl := parallelTestTemplate(t)
	specs := parallelTestSpecs(50)

	serialDir := t.TempDir()
	serialStats, err := renderPageSpecsConcurrent(tmpl, serialDir, specs, 1)
	if err != nil {
		t.Fatalf("serial render: %v", err)
	}

	for _, workers := range []int{2, 4, 8, 64} {
		parallelDir := t.TempDir()
		stats, err := renderPageSpecsConcurrent(tmpl, parallelDir, specs, workers)
		if err != nil {
			t.Fatalf("parallel render (workers=%d): %v", workers, err)
		}
		if stats != serialStats {
			t.Fatalf("workers=%d stats %#v != serial %#v", workers, stats, serialStats)
		}

		got := snapshotDir(t, parallelDir)
		want := snapshotDir(t, serialDir)
		gotKeys := sortedKeys(got)
		wantKeys := sortedKeys(want)
		if len(gotKeys) != len(wantKeys) {
			t.Fatalf("workers=%d produced %d files, serial produced %d", workers, len(gotKeys), len(wantKeys))
		}
		for _, k := range wantKeys {
			if got[k] != want[k] {
				t.Fatalf("workers=%d file %q content mismatch: %q vs %q", workers, k, got[k], want[k])
			}
		}
	}

	if serialStats.Rendered != len(specs) || serialStats.Skipped != 0 {
		t.Fatalf("expected %d rendered, 0 skipped; got %#v", len(specs), serialStats)
	}
}

// TestRenderPageSpecsConcurrent_Stats checks each stat bucket: skip-reason
// pages and unchanged on-disk content count as Skipped; fresh writes count as
// Rendered.
func TestRenderPageSpecsConcurrent_Stats(t *testing.T) {
	tmpl := parallelTestTemplate(t)
	outputDir := t.TempDir()

	specs := []PageSpec{
		{TemplateCandidates: []string{"page"}, OutputPath: "a/index.html", Data: "a"},
		{TemplateCandidates: []string{"page"}, OutputPath: "b/index.html", Data: "b"},
		{TemplateCandidates: []string{"page"}, OutputPath: "c/index.html", Data: "c", SkipReason: "unchanged-source"},
	}

	first, err := renderPageSpecsConcurrent(tmpl, outputDir, specs, 4)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	// a and b render; c is skipped via SkipReason.
	if first.Rendered != 2 || first.Skipped != 1 {
		t.Fatalf("first build expected rendered=2 skipped=1, got %#v", first)
	}

	// Second build: a and b are already on disk with identical content, so they
	// are skipped by the content comparison; c is still SkipReason-skipped.
	second, err := renderPageSpecsConcurrent(tmpl, outputDir, specs, 4)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if second.Rendered != 0 || second.Skipped != 3 {
		t.Fatalf("second build expected rendered=0 skipped=3, got %#v", second)
	}
}

// TestRenderPageSpecsConcurrent_PropagatesError ensures a render failure on any
// worker surfaces a non-nil error equivalent to the serial path.
func TestRenderPageSpecsConcurrent_PropagatesError(t *testing.T) {
	tmpl := parallelTestTemplate(t)

	specs := parallelTestSpecs(20)
	// Inject a page whose template candidate does not exist.
	specs = append(specs, PageSpec{
		TemplateCandidates: []string{"does-not-exist"},
		OutputPath:         "broken/index.html",
		Data:               "x",
	})

	serialErr := renderPageSpecs(tmpl, t.TempDir(), specs)
	if serialErr == nil {
		t.Fatal("expected serial render to fail on missing template")
	}

	_, err := renderPageSpecsConcurrent(tmpl, t.TempDir(), specs, 8)
	if err == nil {
		t.Fatal("expected concurrent render to fail on missing template")
	}
	// Both paths route through pageRenderError, so the message should match.
	if err.Error() != serialErr.Error() {
		t.Fatalf("concurrent error %q != serial error %q", err.Error(), serialErr.Error())
	}
}

// TestRenderPageSpecsConcurrent_EmptyAndSingle covers the serial fallback
// boundaries (no pages, one page) without spinning up workers.
func TestRenderPageSpecsConcurrent_EmptyAndSingle(t *testing.T) {
	tmpl := parallelTestTemplate(t)

	stats, err := renderPageSpecsConcurrent(tmpl, t.TempDir(), nil, 8)
	if err != nil {
		t.Fatalf("empty render: %v", err)
	}
	if stats.Rendered != 0 || stats.Skipped != 0 {
		t.Fatalf("empty render expected zero stats, got %#v", stats)
	}

	stats, err = renderPageSpecsConcurrent(tmpl, t.TempDir(), parallelTestSpecs(1), 8)
	if err != nil {
		t.Fatalf("single render: %v", err)
	}
	if stats.Rendered != 1 || stats.Skipped != 0 {
		t.Fatalf("single render expected rendered=1, got %#v", stats)
	}
}
