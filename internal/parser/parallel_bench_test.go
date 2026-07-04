package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// generateBenchmarkPosts seeds a temp directory with `count` markdown posts
// and returns the directory path. Posts have realistic-ish front matter and
// a short body so the benchmark measures parse + render cost, not just I/O.
func generateBenchmarkPosts(b *testing.B, count int) string {
	b.Helper()
	dir := b.TempDir()
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("2026-01-%02d-post-%03d.md", (i%28)+1, i))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatalf("mkdir: %v", err)
		}
		body := fmt.Sprintf("---\ntitle: \"Post %d\"\ndate: 2026-01-%02dT10:00:00+08:00\ntags: [\"go\",\"bench\"]\n---\n\n# Post %d\n\nThis is the body of post number %d. It has some text to render.\n", i, (i%28)+1, i, i)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
	return dir
}

// BenchmarkParsePosts_Concurrency sweeps the worker count to quantify the
// parallel-parse speedup. jobs=1 is the serial baseline; 0 means auto.
func BenchmarkParsePosts_Concurrency(b *testing.B) {
	for _, n := range []int{100, 500} {
		dir := generateBenchmarkPosts(b, n)
		for _, jobs := range []int{1, 2, 4, 0} {
			label := fmt.Sprintf("posts=%d/jobs=%d", n, jobs)
			b.Run(label, func(b *testing.B) {
				opts := DefaultRenderOptions()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := ParsePostsWithOptionsConcurrent(dir, opts, jobs); err != nil {
						b.Fatalf("parse: %v", err)
					}
				}
			})
		}
	}
}
