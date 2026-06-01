package generator

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type renderer interface {
	Lookup(name string) *template.Template
	ExecuteTemplate(wr io.Writer, name string, data interface{}) error
}

type PageSpec struct {
	TemplateCandidates []string
	OutputPath         string
	Title              string
	Data               interface{}
	// SkipReason, when non-empty, marks the page as eligible to be skipped
	// during rendering — typically because the manifest indicates the source
	// content has not changed since the previous build. The render pipeline
	// counts these toward the Skipped stat and does not touch the output
	// file.
	SkipReason string
}

func renderPageSpecs(tmpl renderer, outputDir string, pages []PageSpec) error {
	_, err := renderPageSpecsWithResult(tmpl, outputDir, pages)
	return err
}

func renderPageSpecsWithResult(tmpl renderer, outputDir string, pages []PageSpec) (PageRenderStats, error) {
	var stats PageRenderStats
	for _, page := range pages {
		wrote, err := renderSinglePage(tmpl, outputDir, page)
		if err != nil {
			return stats, err
		}
		if wrote {
			stats.Rendered++
		} else {
			stats.Skipped++
		}
	}

	return stats, nil
}

// renderPageSpecsConcurrent renders pages across a fixed set of workers. It is
// behaviourally equivalent to renderPageSpecsWithResult: each page writes its
// own independent output path and the shared *template.Template is only
// executed (read-only). Output is deterministic because content does not depend
// on render order.
//
// Pages are striped across workers (worker w renders indices w, w+workers, …)
// rather than dispatched one at a time over a channel. Per-page work is small
// and uniform, so striping avoids per-page channel handoff and lock contention:
// each worker accumulates a local PageRenderStats and the totals are summed
// once at the end. A shared atomic flag lets workers stop promptly after the
// first error.
//
// When concurrency <= 1 (or there is at most one page) it falls back to the
// serial path.
func renderPageSpecsConcurrent(tmpl renderer, outputDir string, pages []PageSpec, concurrency int) (PageRenderStats, error) {
	if concurrency <= 1 || len(pages) <= 1 {
		return renderPageSpecsWithResult(tmpl, outputDir, pages)
	}

	workers := min(concurrency, len(pages))

	var (
		failed   atomic.Bool
		firstErr error
		once     sync.Once
	)
	fail := func(err error) {
		once.Do(func() { firstErr = err })
		failed.Store(true)
	}

	localStats := make([]PageRenderStats, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			stats := &localStats[w]
			for i := w; i < len(pages); i += workers {
				if failed.Load() {
					return
				}
				wrote, err := renderSinglePage(tmpl, outputDir, pages[i])
				if err != nil {
					fail(err)
					return
				}
				if wrote {
					stats.Rendered++
				} else {
					stats.Skipped++
				}
			}
		}(w)
	}
	wg.Wait()

	if firstErr != nil {
		return PageRenderStats{}, firstErr
	}
	var total PageRenderStats
	for _, s := range localStats {
		total.Rendered += s.Rendered
		total.Skipped += s.Skipped
	}
	return total, nil
}

// renderSinglePage renders one page spec to its output file. It reports whether
// it actually wrote the file (rendered) versus skipped it (SkipReason set, or
// the on-disk content already matches). It is safe to call concurrently for
// distinct pages: the template is executed read-only into a private buffer and
// each page targets its own output path.
func renderSinglePage(tmpl renderer, outputDir string, page PageSpec) (rendered bool, err error) {
	if page.SkipReason != "" {
		return false, nil
	}
	templateName, err := resolveTemplateName(tmpl, page.TemplateCandidates)
	if err != nil {
		return false, pageRenderError(page, "", err)
	}

	outputPath, err := safeOutputPath(outputDir, page.OutputPath)
	if err != nil {
		return false, pageRenderError(page, templateName, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return false, pageRenderError(page, templateName, err)
	}
	content, err := renderTemplateContent(tmpl, templateName, page.Data)
	if err != nil {
		return false, pageRenderError(page, templateName, err)
	}
	if same, err := fileHasContent(outputPath, content); err != nil {
		return false, pageRenderError(page, templateName, err)
	} else if same {
		return false, nil
	}
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return false, pageRenderError(page, templateName, err)
	}
	return true, nil
}

func renderTemplateContent(tmpl renderer, name string, data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fileHasContent(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bytes.Equal(existing, content), nil
}

func resolveTemplateName(tmpl renderer, candidates []string) (string, error) {
	for _, candidate := range candidates {
		if tmpl.Lookup(candidate) != nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no template found for candidates: %s", strings.Join(candidates, ", "))
}

func pageRenderError(page PageSpec, templateName string, err error) error {
	parts := []string{"render page"}
	if page.OutputPath != "" {
		parts = append(parts, fmt.Sprintf("output=%q", page.OutputPath))
	}
	if page.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%q", page.Title))
	}
	if templateName != "" {
		parts = append(parts, fmt.Sprintf("template=%q", templateName))
	} else if len(page.TemplateCandidates) > 0 {
		parts = append(parts, fmt.Sprintf("templates=%q", strings.Join(page.TemplateCandidates, ",")))
	}

	return fmt.Errorf("%s: %w", strings.Join(parts, " "), err)
}
