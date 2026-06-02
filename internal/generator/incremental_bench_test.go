package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mengbin92/gobin/internal/parser"
)

// generateIncrementalBenchmarkPosts seeds a temporary site with `count`
// markdown posts plus the templates required to render them, and returns the
// site directory, output directory, and parser.Post slice with FilePath set.
//
// The benchmarks use this to compare:
//
//   - BenchmarkBuildFull: clean output every run, full render every time.
//   - BenchmarkBuildIncremental_NoChanges: prime once, then measure repeated
//     no-change incremental builds (the warm-cache hit path).
func generateIncrementalBenchmarkPosts(b *testing.B, count int) (string, string, []*parser.Post) {
	b.Helper()
	tmpDir := b.TempDir()
	siteDir := filepath.Join(tmpDir, "site")
	outputDir := filepath.Join(siteDir, "public")

	for _, spec := range []struct{ path, content string }{
		{filepath.Join(siteDir, "templates", "_default", "base.html"), `{{ define "base" }}{{ template "main" . }}{{ end }}`},
		{filepath.Join(siteDir, "templates", "_default", "list.html"), `{{ define "main" }}list{{ end }}{{ define "listMain" }}{{ range .Posts }}{{ .Title }};{{ end }}{{ end }}{{ define "listPage" }}{{ template "base" . }}{{ end }}`},
		{filepath.Join(siteDir, "templates", "_default", "single.html"), `{{ define "main" }}single{{ end }}{{ define "singleMain" }}{{ .Post.Title }}{{ end }}{{ define "singlePage" }}{{ template "base" . }}{{ end }}`},
		{filepath.Join(siteDir, "templates", "_default", "taxonomy.html"), `{{ define "taxonomyTermsMain" }}{{ end }}{{ define "taxonomyMain" }}{{ end }}{{ define "taxonomyTermsPage" }}{{ template "base" . }}{{ end }}{{ define "taxonomyPage" }}{{ template "base" . }}{{ end }}`},
		{filepath.Join(siteDir, "templates", "_default", "404.html"), `{{ define "notFoundMain" }}404{{ end }}{{ define "notFoundPage" }}{{ template "base" . }}{{ end }}`},
	} {
		if err := os.MkdirAll(filepath.Dir(spec.path), 0755); err != nil {
			b.Fatalf("mkdir %s: %v", spec.path, err)
		}
		if err := os.WriteFile(spec.path, []byte(spec.content), 0644); err != nil {
			b.Fatalf("write %s: %v", spec.path, err)
		}
	}

	posts := make([]*parser.Post, count)
	for i := 0; i < count; i++ {
		postPath := filepath.Join(siteDir, "_posts", fmt.Sprintf("2026-01-%02d-post-%d.md", (i%28)+1, i))
		if err := os.MkdirAll(filepath.Dir(postPath), 0755); err != nil {
			b.Fatalf("mkdir _posts: %v", err)
		}
		if err := os.WriteFile(postPath, []byte(fmt.Sprintf("post %d body", i)), 0644); err != nil {
			b.Fatalf("write %s: %v", postPath, err)
		}
		posts[i] = &parser.Post{
			Title:       fmt.Sprintf("Post %d", i),
			Slug:        fmt.Sprintf("post-%d", i),
			URL:         fmt.Sprintf("/post-%d/", i),
			FilePath:    postPath,
			ContentHTML: fmt.Sprintf("<p>body %d</p>", i),
			Tags:        []string{"go"},
		}
	}
	return siteDir, outputDir, posts
}

func chdirForBench(b *testing.B, dir string) {
	b.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		b.Fatalf("chdir %s: %v", dir, err)
	}
	b.Cleanup(func() { _ = os.Chdir(oldWd) })
}

func BenchmarkBuildFull(b *testing.B) {
	for _, n := range []int{10, 100} {
		b.Run(fmt.Sprintf("posts=%d", n), func(b *testing.B) {
			siteDir, outputDir, posts := generateIncrementalBenchmarkPosts(b, n)
			cfg := incrementalCfg()
			chdirForBench(b, siteDir)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := GenerateWithOptions(posts, nil, cfg, GenerationOptions{
					OutputDir:   outputDir,
					CleanOutput: true,
				}); err != nil {
					b.Fatalf("build: %v", err)
				}
			}
		})
	}
}

// BenchmarkBuildFull_Concurrency isolates the page-render phase (posts are
// pre-parsed, so no markdown work happens here) and sweeps the worker count to
// quantify the parallel-build speedup. concurrency=1 is the serial baseline;
// 0 means auto (runtime.NumCPU()).
func BenchmarkBuildFull_Concurrency(b *testing.B) {
	for _, n := range []int{100, 500} {
		for _, jobs := range []int{1, 2, 4, 0} {
			label := fmt.Sprintf("posts=%d/jobs=%d", n, jobs)
			b.Run(label, func(b *testing.B) {
				siteDir, outputDir, posts := generateIncrementalBenchmarkPosts(b, n)
				cfg := incrementalCfg()
				chdirForBench(b, siteDir)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := GenerateWithOptions(posts, nil, cfg, GenerationOptions{
						OutputDir:   outputDir,
						CleanOutput: true,
						Concurrency: jobs,
					}); err != nil {
						b.Fatalf("build: %v", err)
					}
				}
			})
		}
	}
}

func BenchmarkBuildIncremental_NoChanges(b *testing.B) {
	for _, n := range []int{10, 100} {
		b.Run(fmt.Sprintf("posts=%d", n), func(b *testing.B) {
			siteDir, outputDir, posts := generateIncrementalBenchmarkPosts(b, n)
			cfg := incrementalCfg()
			chdirForBench(b, siteDir)

			// Prime the manifest with a full build first.
			if _, err := GenerateWithOptions(posts, nil, cfg, GenerationOptions{
				OutputDir:   outputDir,
				CleanOutput: true,
			}); err != nil {
				b.Fatalf("prime: %v", err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := GenerateWithOptions(posts, nil, cfg, GenerationOptions{
					OutputDir:   outputDir,
					Incremental: true,
				}); err != nil {
					b.Fatalf("incremental build: %v", err)
				}
			}
		})
	}
}
