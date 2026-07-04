package generator

import (
	"os"
	"strings"
	"testing"
)

func TestRewriteImageReferences_MultiFormat(t *testing.T) {
	sources := map[string]ImageSourceRewrite{
		"/img/cover.jpg": {
			Widths:  []int{480, 800},
			Formats: []string{"jpg", "png"},
			Outputs: map[string]map[string]string{
				"480w": {"jpg": "/img/cover-480w.jpg", "png": "/img/cover-480w.png"},
				"800w": {"jpg": "/img/cover-800w.jpg", "png": "/img/cover-800w.png"},
			},
			Sizes: "(max-width: 800px) 100vw, 800px",
		},
	}
	html := `<p><img src="/img/cover.jpg" alt="Cover" class="hero"></p>`
	got, found, replaced := rewriteImageReferences(html, sources)
	if found != 1 || replaced != 1 {
		t.Fatalf("found=%d replaced=%d, want 1/1\ngot: %s", found, replaced, got)
	}
	if !strings.Contains(got, "<picture>") {
		t.Errorf("expected <picture> wrapper, got: %s", got)
	}
	if !strings.Contains(got, `<source type="image/jpg"`) {
		t.Errorf("expected <source type=image/jpg>, got: %s", got)
	}
	if !strings.Contains(got, `<source type="image/png"`) {
		t.Errorf("expected <source type=image/png>, got: %s", got)
	}
	if !strings.Contains(got, `alt="Cover"`) {
		t.Errorf("expected alt attribute preserved, got: %s", got)
	}
	if !strings.Contains(got, `class="hero"`) {
		t.Errorf("expected class attribute preserved, got: %s", got)
	}
	if !strings.Contains(got, `loading="lazy"`) {
		t.Errorf("expected loading=lazy on fallback, got: %s", got)
	}
	if !strings.Contains(got, `sizes="(max-width: 800px) 100vw, 800px"`) {
		t.Errorf("expected sizes attribute, got: %s", got)
	}
}

func TestRewriteImageReferences_SingleFormat(t *testing.T) {
	sources := map[string]ImageSourceRewrite{
		"/img/cover.jpg": {
			Widths:  []int{480, 800},
			Formats: []string{"jpg"},
			Outputs: map[string]map[string]string{
				"480w": {"jpg": "/img/cover-480w.jpg"},
				"800w": {"jpg": "/img/cover-800w.jpg"},
			},
			Sizes: "(max-width: 800px) 100vw, 800px",
		},
	}
	html := `<img src="/img/cover.jpg" alt="x">`
	got, found, replaced := rewriteImageReferences(html, sources)
	if found != 1 || replaced != 1 {
		t.Fatalf("found=%d replaced=%d", found, replaced)
	}
	if strings.Contains(got, "<picture>") {
		t.Errorf("single-format path should not wrap in <picture>, got: %s", got)
	}
	if !strings.Contains(got, `srcset="/img/cover-480w.jpg 480w, /img/cover-800w.jpg 800w"`) {
		t.Errorf("expected srcset, got: %s", got)
	}
}

func TestRewriteImageReferences_LeavesUnrelatedImagesAlone(t *testing.T) {
	sources := map[string]ImageSourceRewrite{
		"/img/known.jpg": {
			Widths:  []int{480},
			Formats: []string{"jpg"},
			Outputs: map[string]map[string]string{
				"480w": {"jpg": "/img/known-480w.jpg"},
			},
		},
	}
	html := `<p><img src="/img/known.jpg"><img src="/img/unknown.jpg"></p>`
	got, found, replaced := rewriteImageReferences(html, sources)
	if found != 1 || replaced != 1 {
		t.Fatalf("found=%d replaced=%d, want 1/1 (only known.jpg should match)", found, replaced)
	}
	if !strings.Contains(got, "/img/unknown.jpg") {
		t.Errorf("unknown image should be left alone, got: %s", got)
	}
}

func TestRewriteImageReferences_EmptySources(t *testing.T) {
	html := `<p><img src="/img/x.jpg"></p>`
	got, found, replaced := rewriteImageReferences(html, nil)
	if found != 0 || replaced != 0 || got != html {
		t.Fatalf("nil sources should be a no-op: got %q", got)
	}
}

func TestPostprocessHTML_WithImageSources(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/index.html", []byte(`<img src="/img/cover.jpg" alt="x">`), 0644); err != nil {
		t.Fatal(err)
	}
	stats, err := PostprocessHTML(PostprocessOptions{
		OutputDir: tmp,
		ImageSources: map[string]ImageSourceRewrite{
			"/img/cover.jpg": {
				Widths:  []int{480, 800},
				Formats: []string{"jpg"},
				Outputs: map[string]map[string]string{
					"480w": {"jpg": "/img/cover-480w.jpg"},
					"800w": {"jpg": "/img/cover-800w.jpg"},
				},
				Sizes: "(max-width: 800px) 100vw, 800px",
			},
		},
	})
	if err != nil {
		t.Fatalf("PostprocessHTML: %v", err)
	}
	if stats.ImageReferencesFound != 1 {
		t.Errorf("ImageReferencesFound = %d, want 1", stats.ImageReferencesFound)
	}
	if stats.ImageReferencesRewritten != 1 {
		t.Errorf("ImageReferencesRewritten = %d, want 1", stats.ImageReferencesRewritten)
	}
}
