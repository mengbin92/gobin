package commands

import (
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// changeKind classifies a watched filesystem path by how it should drive an
// incremental serve rebuild.
type changeKind int

const (
	// changeStructural forces a full reload: config files, templates, theme
	// assets/layouts, or any non-markdown file inside a content directory. A
	// full reload re-parses every source and refreshes the whole cache, which
	// matches the original (pre-optimization) serve behavior.
	changeStructural changeKind = iota
	// changeContent is a markdown post under cfg.ContentDir: reparse only that
	// file (or drop it if it no longer exists).
	changeContent
	// changePage is a markdown standalone page under cfg.PageDir.
	changePage
	// changeStatic is a file under cfg.StaticDir: the parsed content cache is
	// untouched and the generator's incremental asset copy handles it.
	changeStatic
)

// classifyChange maps a watched path to the rebuild strategy it requires. It is
// a pure function (no I/O) so the file's existence is irrelevant here — the
// incremental loader decides reparse-vs-drop by stat'ing the path later. The
// directory rules mirror watchPaths in serve_watcher.go and the markdown
// extension check in parser.go.
func classifyChange(path string, cfg *config.Config) changeKind {
	if cfg == nil {
		return changeStructural
	}

	clean := filepath.Clean(path)
	switch filepath.Base(clean) {
	case "config.yaml", "config.yml", "_config.yml", "_config.yaml":
		return changeStructural
	}

	if isWithin(clean, "templates") {
		return changeStructural
	}
	if cfg.Theme != "" && cfg.ThemesDir != "" {
		if isWithin(clean, filepath.Join(cfg.ThemesDir, cfg.Theme)) {
			return changeStructural
		}
	}

	isMarkdown := isMarkdownPath(clean)
	if isWithin(clean, cfg.ContentDir) {
		if isMarkdown {
			return changeContent
		}
		return changeStructural
	}
	if isWithin(clean, cfg.PageDir) {
		if isMarkdown {
			return changePage
		}
		return changeStructural
	}
	if isWithin(clean, cfg.StaticDir) {
		return changeStatic
	}

	// Anything we cannot place is treated as structural so a full reload keeps
	// output correct rather than silently missing a change.
	return changeStructural
}

func isMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// isWithin reports whether path is dir itself or nested under it. Both
// arguments are cleaned so "./content" and "content" compare equal.
func isWithin(path, dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// changeSet accumulates the paths reported by the file watcher between debounced
// rebuilds. It is written by the watcher goroutine and drained by the rebuild
// (timer) goroutine, so every method is guarded by a mutex.
type changeSet struct {
	mu        sync.Mutex
	content   map[string]struct{}
	pages     map[string]struct{}
	forceFull bool
}

// add records a single watched path. A structural change latches forceFull so a
// later content change in the same window cannot downgrade the rebuild.
func (c *changeSet) add(path string, kind changeKind) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch kind {
	case changeContent:
		if c.content == nil {
			c.content = make(map[string]struct{})
		}
		c.content[filepath.Clean(path)] = struct{}{}
	case changePage:
		if c.pages == nil {
			c.pages = make(map[string]struct{})
		}
		c.pages[filepath.Clean(path)] = struct{}{}
	case changeStatic:
		// Static-only changes need no content reparse; the generator's
		// incremental asset copy handles them. Nothing to record.
	default:
		c.forceFull = true
	}
}

// drain returns the accumulated content/page paths plus the forceFull flag and
// resets the set for the next window.
func (c *changeSet) drain() (contentPaths, pagePaths []string, forceFull bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	contentPaths = keysOf(c.content)
	pagePaths = keysOf(c.pages)
	forceFull = c.forceFull

	c.content = nil
	c.pages = nil
	c.forceFull = false
	return contentPaths, pagePaths, forceFull
}

func keysOf(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// contentCache holds the most recent successful parse of every post and page
// source, keyed by FilePath, so a watch-driven rebuild can reparse only the
// files that changed. It is touched only by the rebuild goroutine.
type contentCache struct {
	cfg    *config.Config
	opts   parser.RenderOptions
	posts  map[string]*parser.Post
	pages  map[string]*parser.Page
	primed bool
}

// refreshAll replaces the entire cache from a full site load. Called for the
// initial prime and whenever a structural change forces a full reload.
func (c *contentCache) refreshAll(input *siteBuildInput) error {
	opts, err := renderOptionsFromConfig(input.cfg)
	if err != nil {
		return err
	}
	c.cfg = input.cfg
	c.opts = opts
	c.posts = make(map[string]*parser.Post, len(input.posts))
	for _, post := range input.posts {
		if post == nil || post.FilePath == "" {
			continue
		}
		c.posts[filepath.Clean(post.FilePath)] = post
	}
	c.pages = make(map[string]*parser.Page, len(input.pages))
	for _, page := range input.pages {
		if page == nil || page.FilePath == "" {
			continue
		}
		c.pages[filepath.Clean(page.FilePath)] = page
	}
	c.primed = true
	return nil
}

// assemble builds a siteBuildInput from the cache. Entries are emitted in
// lexical FilePath order so output matches a full filepath.WalkDir parse
// (the generator re-sorts posts by date with a non-stable sort, and pages are
// never sorted, so input order still determines tie-breaks and page order).
//
// Each entry is a shallow struct copy: the generator's preparePosts mutates
// URL/Content/ContentHTML/Summary in place, and handing out copies keeps the
// cached parse pristine across rebuilds.
func (c *contentCache) assemble() *siteBuildInput {
	postKeys := make([]string, 0, len(c.posts))
	for k := range c.posts {
		postKeys = append(postKeys, k)
	}
	sort.Strings(postKeys)
	posts := make([]*parser.Post, 0, len(postKeys))
	for _, k := range postKeys {
		clone := *c.posts[k]
		posts = append(posts, &clone)
	}

	pageKeys := make([]string, 0, len(c.pages))
	for k := range c.pages {
		pageKeys = append(pageKeys, k)
	}
	sort.Strings(pageKeys)
	pages := make([]*parser.Page, 0, len(pageKeys))
	for _, k := range pageKeys {
		clone := *c.pages[k]
		pages = append(pages, &clone)
	}

	return &siteBuildInput{cfg: c.cfg, posts: posts, pages: pages}
}

// newIncrementalLoader returns a loader with the same signature as
// loadSiteBuildInput, so it drops into serveBuilder unchanged. On each call it
// drains the change set and either does a full reload (cache unprimed, a
// structural change, or no recorded content/page changes) or reparses only the
// changed markdown files, reusing the cache for everything else.
//
// A changed path that no longer exists on disk is dropped from the cache; this
// covers deletes and editor atomic-saves (rename-over) uniformly — the final
// on-disk state wins. The cache is mutated only after every reparse succeeds,
// so a transient parse error (e.g. a half-written file) leaves the previous
// good state intact for the next attempt.
// report, when non-nil, is invoked after each load with the number of sources
// reparsed and the number reused from the cache (full=true for a full reload).
// It exists purely for verbose dev-server logging.
func newIncrementalLoader(cache *contentCache, changes *changeSet, fullLoad func() (*siteBuildInput, error), report func(reparsed, reused int, full bool)) func() (*siteBuildInput, error) {
	return func() (*siteBuildInput, error) {
		contentPaths, pagePaths, forceFull := changes.drain()

		if !cache.primed || forceFull || (len(contentPaths) == 0 && len(pagePaths) == 0) {
			input, err := fullLoad()
			if err != nil {
				return nil, err
			}
			if err := cache.refreshAll(input); err != nil {
				return nil, err
			}
			if report != nil {
				report(len(cache.posts)+len(cache.pages), 0, true)
			}
			return cache.assemble(), nil
		}

		// Parse into staging maps first; commit only on full success.
		reparsedPosts := make(map[string]*parser.Post, len(contentPaths))
		droppedPosts := make([]string, 0)
		for _, path := range contentPaths {
			if !fileExists(path) {
				droppedPosts = append(droppedPosts, path)
				continue
			}
			post, err := parser.ParsePostWithOptions(path, cache.opts)
			if err != nil {
				return nil, err
			}
			reparsedPosts[filepath.Clean(path)] = post
		}

		reparsedPages := make(map[string]*parser.Page, len(pagePaths))
		droppedPages := make([]string, 0)
		for _, path := range pagePaths {
			if !fileExists(path) {
				droppedPages = append(droppedPages, path)
				continue
			}
			page, err := parser.ParsePageWithOptions(path, cache.cfg.PageDir, cache.opts)
			if err != nil {
				return nil, err
			}
			reparsedPages[filepath.Clean(path)] = page
		}

		maps.Copy(cache.posts, reparsedPosts)
		for _, k := range droppedPosts {
			delete(cache.posts, filepath.Clean(k))
		}
		maps.Copy(cache.pages, reparsedPages)
		for _, k := range droppedPages {
			delete(cache.pages, filepath.Clean(k))
		}

		if report != nil {
			reparsed := len(reparsedPosts) + len(reparsedPages)
			report(reparsed, len(cache.posts)+len(cache.pages)-reparsed, false)
		}
		return cache.assemble(), nil
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
