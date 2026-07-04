package parser

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// parseConcurrencyCap bounds the worker count chosen for "auto" (Concurrency
// <= 0). Markdown parsing mixes file I/O (os.ReadFile) with CPU work
// (goldmark Convert), so throughput peaks at a handful of workers and
// regresses past that as filesystem contention outweighs the gains. The cap
// matches autoConcurrencyCap in the generator package so --jobs 0 selects the
// same effective parallelism for both the parse and render phases.
const parseConcurrencyCap = 4

// normalizeParseConcurrency resolves a requested worker count into an effective
// one. A non-positive value means "auto", which maps to the number of CPUs
// capped at parseConcurrencyCap.
func normalizeParseConcurrency(concurrency int) int {
	if concurrency <= 0 {
		return min(runtime.NumCPU(), parseConcurrencyCap)
	}
	return concurrency
}

// parsePostFilesConcurrent parses files in parallel and returns posts in the
// same order as files (which filepath.WalkDir yields lexicographically), so
// the result is byte-for-byte identical to a serial parse.
//
// It is behaviourally equivalent to a serial loop calling ParsePostWithOptions
// on each file: each goroutine parses its own files into pre-allocated slice
// slots (stripe assignment), the shared RenderOptions is read-only, and the
// first error is captured via sync.Once + an atomic flag so other workers stop
// promptly.
//
// When concurrency <= 1 (or there is at most one file) it falls back to the
// serial path.
func parsePostFilesConcurrent(files []string, opts RenderOptions, concurrency int) ([]*Post, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if concurrency <= 1 || len(files) <= 1 {
		posts := make([]*Post, 0, len(files))
		for _, path := range files {
			post, err := ParsePostWithOptions(path, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", path, err)
			}
			if post != nil {
				posts = append(posts, post)
			}
		}
		return posts, nil
	}

	workers := min(concurrency, len(files))
	results := make([]*Post, len(files))

	var (
		failed   atomic.Bool
		firstErr error
		once     sync.Once
	)
	fail := func(err error) {
		once.Do(func() { firstErr = err })
		failed.Store(true)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := w; i < len(files); i += workers {
				if failed.Load() {
					return
				}
				post, err := ParsePostWithOptions(files[i], opts)
				if err != nil {
					fail(fmt.Errorf("failed to parse %s: %w", files[i], err))
					return
				}
				results[i] = post
			}
		}(w)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Compact nil slots (ParsePostWithOptions may return nil for a post that
	// is filtered out) while preserving file order.
	posts := make([]*Post, 0, len(results))
	for _, post := range results {
		if post != nil {
			posts = append(posts, post)
		}
	}
	return posts, nil
}

// parsePageFilesConcurrent is the page analogue of parsePostFilesConcurrent.
// baseDir is forwarded to ParsePageWithOptions for URL derivation.
func parsePageFilesConcurrent(files []string, baseDir string, opts RenderOptions, concurrency int) ([]*Page, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if concurrency <= 1 || len(files) <= 1 {
		pages := make([]*Page, 0, len(files))
		for _, path := range files {
			page, err := ParsePageWithOptions(path, baseDir, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", path, err)
			}
			pages = append(pages, page)
		}
		return pages, nil
	}

	workers := min(concurrency, len(files))
	results := make([]*Page, len(files))

	var (
		failed   atomic.Bool
		firstErr error
		once     sync.Once
	)
	fail := func(err error) {
		once.Do(func() { firstErr = err })
		failed.Store(true)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := w; i < len(files); i += workers {
				if failed.Load() {
					return
				}
				page, err := ParsePageWithOptions(files[i], baseDir, opts)
				if err != nil {
					fail(fmt.Errorf("failed to parse %s: %w", files[i], err))
					return
				}
				results[i] = page
			}
		}(w)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	pages := make([]*Page, 0, len(results))
	for _, page := range results {
		if page != nil {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

// collectMarkdownFiles walks dir and returns the lexicographically sorted list
// of .md / .markdown file paths, matching the order filepath.WalkDir yields.
// It is shared by ParsePostsWithOptionsConcurrent and
// ParsePagesWithOptionsConcurrent so both phases collect-then-parse in two
// steps, keeping the parallel parse path uniform.
func collectMarkdownFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
