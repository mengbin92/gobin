package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/parser"
)

func TestClassifyChange(t *testing.T) {
	cfg := config.Normalize(&config.Config{Theme: "mytheme"})

	cases := []struct {
		name string
		path string
		want changeKind
	}{
		{"content post", filepath.Join("_posts", "2026-05-01-x.md"), changeContent},
		{"content markdown ext", filepath.Join("_posts", "x.markdown"), changeContent},
		{"standalone page", filepath.Join("pages", "about.md"), changePage},
		{"static asset", filepath.Join("assets", "css", "style.css"), changeStatic},
		{"config yaml", "config.yaml", changeStructural},
		{"config yml underscore", "_config.yml", changeStructural},
		{"template", filepath.Join("templates", "_default", "single.html"), changeStructural},
		{"site shortcode", filepath.Join("templates", "shortcodes", "figure.html"), changeStructural},
		{"jekyll layout", filepath.Join("_layouts", "post.html"), changeStructural},
		{"jekyll include", filepath.Join("_includes", "header.html"), changeStructural},
		{"theme layout", filepath.Join("themes", "mytheme", "layouts", "x.html"), changeStructural},
		{"theme shortcode", filepath.Join("themes", "mytheme", "layouts", "shortcodes", "figure.html"), changeStructural},
		{"non-md under content", filepath.Join("_posts", "data.json"), changeStructural},
		{"unknown path", filepath.Join("somewhere", "else.md"), changeStructural},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyChange(tc.path, cfg); got != tc.want {
				t.Fatalf("classifyChange(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestChangeSet_DrainClearsAndMergesForceFull(t *testing.T) {
	changes := &changeSet{}
	changes.add(filepath.Join("_posts", "a.md"), changeContent)
	changes.add(filepath.Join("_posts", "a.md"), changeContent) // dedup
	changes.add(filepath.Join("pages", "p.md"), changePage)
	changes.add(filepath.Join("assets", "s.css"), changeStatic) // recorded as nothing

	contentPaths, pagePaths, forceFull := changes.drain()
	if len(contentPaths) != 1 || len(pagePaths) != 1 {
		t.Fatalf("expected 1 content + 1 page path, got %v / %v", contentPaths, pagePaths)
	}
	if forceFull {
		t.Fatal("expected forceFull=false for content-only changes")
	}

	// A structural change latches forceFull and survives a later content add.
	changes.add(filepath.Join("templates", "x.html"), changeStructural)
	changes.add(filepath.Join("_posts", "b.md"), changeContent)
	_, _, forceFull = changes.drain()
	if !forceFull {
		t.Fatal("expected forceFull=true after a structural change")
	}

	// Drain resets state.
	contentPaths, pagePaths, forceFull = changes.drain()
	if len(contentPaths) != 0 || len(pagePaths) != 0 || forceFull {
		t.Fatalf("expected empty set after drain, got %v / %v / %v", contentPaths, pagePaths, forceFull)
	}
}

func TestContentCache_AssembleIsLexicalByFilePath(t *testing.T) {
	cache := &contentCache{
		posts: map[string]*parser.Post{
			"_posts/c.md": {FilePath: "_posts/c.md", Title: "C"},
			"_posts/a.md": {FilePath: "_posts/a.md", Title: "A"},
			"_posts/b.md": {FilePath: "_posts/b.md", Title: "B"},
		},
		pages: map[string]*parser.Page{
			"pages/z.md": {FilePath: "pages/z.md", Title: "Z"},
			"pages/y.md": {FilePath: "pages/y.md", Title: "Y"},
		},
	}

	input := cache.assemble()
	gotPosts := []string{input.posts[0].FilePath, input.posts[1].FilePath, input.posts[2].FilePath}
	wantPosts := []string{"_posts/a.md", "_posts/b.md", "_posts/c.md"}
	for i := range wantPosts {
		if gotPosts[i] != wantPosts[i] {
			t.Fatalf("posts not lexical: got %v, want %v", gotPosts, wantPosts)
		}
	}
	if input.pages[0].FilePath != "pages/y.md" || input.pages[1].FilePath != "pages/z.md" {
		t.Fatalf("pages not lexical: %v", []string{input.pages[0].FilePath, input.pages[1].FilePath})
	}

	// assemble must hand out copies, not the cached pointers, so the generator's
	// in-place mutation cannot corrupt the cache.
	if input.posts[0] == cache.posts["_posts/a.md"] {
		t.Fatal("assemble returned the cached pointer instead of a copy")
	}
}

func TestIncrementalLoader_ReparsesOnlyChangedPaths(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)
	writePost(t, siteDir, "2026-05-02-second.md", "Second Title", "Second body")
	writePost(t, siteDir, "2026-05-03-third.md", "Third Title", "Third body")
	chdirForTest(t, siteDir)

	cache := &contentCache{}
	changes := &changeSet{}
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)

	// First call primes the cache via a full load.
	if _, err := loader(); err != nil {
		t.Fatalf("prime load failed: %v", err)
	}

	// Mutate TWO posts on disk but only report ONE as changed. The reported one
	// must reflect new content; the unreported one must keep its cached parse.
	secondPath := filepath.Join(siteDir, "_posts", "2026-05-02-second.md")
	thirdPath := filepath.Join(siteDir, "_posts", "2026-05-03-third.md")
	mustWriteFile(t, secondPath, postBody("Second Title", "Second body EDITED"))
	mustWriteFile(t, thirdPath, postBody("Third Title", "Third body EDITED"))

	changes.add(filepath.Join("_posts", "2026-05-02-second.md"), changeContent)

	input, err := loader()
	if err != nil {
		t.Fatalf("incremental load failed: %v", err)
	}

	second := findPost(input.posts, "Second Title")
	third := findPost(input.posts, "Third Title")
	if second == nil || third == nil {
		t.Fatalf("expected both posts present, got %d posts", len(input.posts))
	}
	if !strings.Contains(second.Content, "EDITED") {
		t.Fatalf("reported post was not reparsed: %q", second.Content)
	}
	if strings.Contains(third.Content, "EDITED") {
		t.Fatal("unreported post was reparsed (should have reused cache)")
	}
}

func TestIncrementalLoader_FallsBackToFullOnForceFull(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)
	writePost(t, siteDir, "2026-05-02-second.md", "Second Title", "Second body")
	chdirForTest(t, siteDir)

	cache := &contentCache{}
	changes := &changeSet{}
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)
	if _, err := loader(); err != nil {
		t.Fatalf("prime load failed: %v", err)
	}

	// Edit a post on disk but record a STRUCTURAL change. forceFull must trigger
	// a full reload that picks up the edit even though the post path was not in
	// the change set.
	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2026-05-02-second.md"), postBody("Second Title", "Second body EDITED"))
	changes.add("config.yaml", changeStructural)

	input, err := loader()
	if err != nil {
		t.Fatalf("forceFull load failed: %v", err)
	}
	second := findPost(input.posts, "Second Title")
	if second == nil || !strings.Contains(second.Content, "EDITED") {
		t.Fatal("expected forceFull to fully reload and pick up the edit")
	}
}

func TestIncrementalLoader_FallsBackToFullWhenUnprimed(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)
	chdirForTest(t, siteDir)

	cache := &contentCache{}
	changes := &changeSet{}
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)

	// No recorded changes + unprimed cache → must full load, not return empty.
	input, err := loader()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(input.posts) == 0 {
		t.Fatal("expected unprimed loader to fully load posts")
	}
	if !cache.primed {
		t.Fatal("expected cache to be primed after first load")
	}
}

func TestIncrementalLoader_DroppedFileLeavesContentSet(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)
	writePost(t, siteDir, "2026-05-02-second.md", "Second Title", "Second body")
	chdirForTest(t, siteDir)

	cache := &contentCache{}
	changes := &changeSet{}
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)
	if _, err := loader(); err != nil {
		t.Fatalf("prime load failed: %v", err)
	}

	// Delete a post and report it as a content change. The loader stats the path,
	// finds it gone, and drops it from the assembled input.
	if err := os.Remove(filepath.Join(siteDir, "_posts", "2026-05-02-second.md")); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	changes.add(filepath.Join("_posts", "2026-05-02-second.md"), changeContent)

	input, err := loader()
	if err != nil {
		t.Fatalf("incremental load failed: %v", err)
	}
	if findPost(input.posts, "Second Title") != nil {
		t.Fatal("expected deleted post to be dropped from the assembled input")
	}
	if findPost(input.posts, "Initial Title") == nil {
		t.Fatal("expected the surviving post to remain")
	}
}

// TestIncrementalRebuild_ByteIdenticalToFullRebuild guards the two subtle
// correctness properties of the fast path: handing out struct copies (so the
// generator's in-place mutation does not compound across rebuilds) and lexical
// reassembly order. After editing one post body, a fast-path incremental
// rebuild must produce a publish tree byte-for-byte identical to a full clean
// rebuild of the same edited site.
func TestIncrementalRebuild_ByteIdenticalToFullRebuild(t *testing.T) {
	siteDir := t.TempDir()
	writeServeFixtureSite(t, siteDir)
	writePost(t, siteDir, "2026-05-02-second.md", "Second Title", "Second body")
	writePost(t, siteDir, "2026-05-03-third.md", "Third Title", "Third body")
	chdirForTest(t, siteDir)

	// Prime: full clean build + cache.
	primeInput, err := loadSiteBuildInput()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if _, err := generateSiteWithOptions(primeInput, generator.GenerationOptions{OutputDir: "public", CleanOutput: true}); err != nil {
		t.Fatalf("prime build failed: %v", err)
	}
	cache := &contentCache{}
	if err := cache.refreshAll(primeInput); err != nil {
		t.Fatalf("refreshAll failed: %v", err)
	}

	// Edit only the body of one post (title/date unchanged → aggregates stable).
	mustWriteFile(t, filepath.Join(siteDir, "_posts", "2026-05-02-second.md"), postBody("Second Title", "Second body REVISED"))

	// Fast path: reparse only the edited post, incremental (clean=false).
	changes := &changeSet{}
	changes.add(filepath.Join("_posts", "2026-05-02-second.md"), changeContent)
	loader := newIncrementalLoader(cache, changes, loadSiteBuildInput, nil)
	fastInput, err := loader()
	if err != nil {
		t.Fatalf("fast load failed: %v", err)
	}
	if _, err := generateSiteWithOptions(fastInput, generator.GenerationOptions{OutputDir: "public", Incremental: true}); err != nil {
		t.Fatalf("fast rebuild failed: %v", err)
	}
	fastTree := snapshotTree(t, "public")

	// Reference: full clean rebuild of the edited site.
	fullInput, err := loadSiteBuildInput()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if _, err := generateSiteWithOptions(fullInput, generator.GenerationOptions{OutputDir: "public", CleanOutput: true}); err != nil {
		t.Fatalf("full rebuild failed: %v", err)
	}
	fullTree := snapshotTree(t, "public")

	assertTreesEqual(t, fastTree, fullTree)
}

func TestHandleWatchEvent_RecordsChangePath(t *testing.T) {
	var recorded []string
	record := func(ev fsnotify.Event) { recorded = append(recorded, ev.Name) }

	triggered := handleWatchEvent(fsnotify.Event{Name: "_posts/x.md", Op: fsnotify.Write},
		serveRuntime{stdout: os.Stdout, stderr: os.Stderr}, func(func()) {}, func() {}, record)
	if !triggered {
		t.Fatal("expected write event to trigger")
	}
	if len(recorded) != 1 || recorded[0] != "_posts/x.md" {
		t.Fatalf("expected recorded [_posts/x.md], got %v", recorded)
	}

	// Unsupported events (chmod) must not record.
	recorded = nil
	handleWatchEvent(fsnotify.Event{Name: "_posts/x.md", Op: fsnotify.Chmod},
		serveRuntime{stdout: os.Stdout, stderr: os.Stderr}, func(func()) {}, func() {}, record)
	if len(recorded) != 0 {
		t.Fatalf("expected chmod not to record, got %v", recorded)
	}
}

// --- helpers ---

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
}

func postBody(title, body string) string {
	return "---\ntitle: \"" + title + "\"\ndate: 2026-05-02T10:00:00+08:00\n---\n\n" + body
}

func writePost(t *testing.T, siteDir, name, title, body string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(siteDir, "_posts", name), postBody(title, body))
}

func findPost(posts []*parser.Post, title string) *parser.Post {
	for _, p := range posts {
		if p.Title == title {
			return p
		}
	}
	return nil
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		tree[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return tree
}

func assertTreesEqual(t *testing.T, got, want map[string]string) {
	t.Helper()
	for path, wantHash := range want {
		gotHash, ok := got[path]
		if !ok {
			t.Errorf("fast-path tree missing %s", path)
			continue
		}
		if gotHash != wantHash {
			t.Errorf("content mismatch for %s (fast=%s full=%s)", path, gotHash, wantHash)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("fast-path tree has unexpected extra file %s", path)
		}
	}
}
