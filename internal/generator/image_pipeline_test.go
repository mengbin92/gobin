package generator

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// writeTestJpeg materializes a small but non-trivial JPEG into path
// so the image pipeline has a real source to transform. The content
// (a colored gradient) ensures the re-encoded variant is not byte-
// identical to the source, which exercises the resize/encode path
// instead of the passthrough path.
func writeTestJpeg(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: uint8((x ^ y) * 255 / (w + h)),
				A: 255,
			})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// writeTestPng materializes a small PNG into path. Alpha is fully
// opaque so callers can re-decode the variant and confirm the
// expected pixel survived a round trip.
func writeTestPng(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// makeImagePipelineSite creates a minimal site layout for the
// runImagePipeline integration tests:
//
//	<root>/assets/img/cover.jpg   # referenced via front matter cover
//	<root>/assets/img/inline.jpg  # referenced via ![]() body
//
// The two source images are used by every test in this file. Tests
// that need rendered HTML additionally set up a templates directory
// (see makeImagePipelineSiteWithTemplates) so the full Generate
// path is exercised.
func makeImagePipelineSite(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "assets", "img"), 0755); err != nil {
		t.Fatal(err)
	}
	writeTestJpeg(t, filepath.Join(root, "assets", "img", "cover.jpg"), 1600, 1200)
	writeTestJpeg(t, filepath.Join(root, "assets", "img", "inline.jpg"), 1200, 800)
}

// imagePosts builds a one-post site that references both cover.jpg
// (via front matter) and inline.jpg (via body).
func imagePosts(root string) []*parser.Post {
	return []*parser.Post{{
		FilePath: filepath.Join(root, "posts", "2026-03-20-foo.md"),
		Params: map[string]interface{}{
			"cover": "/img/cover.jpg",
		},
		Content: `Hello.

![inline](/img/inline.jpg)

End.`,
	}}
}

func imagePipelineTestConfig(root string, enabled bool) *config.Config {
	cfg := &config.Config{
		StaticDir:  filepath.Join(root, "assets"),
		PublishDir: filepath.Join(root, "public"),
		BaseURL:    "https://example.com",
	}
	if enabled {
		cfg.Assets = &config.AssetsConfig{
			Images: &config.AssetsImagesConfig{
				Enabled: true,
				Srcset:  []int{480, 800, 1200},
				Sizes:   "(max-width: 800px) 100vw, 800px",
				Formats: []string{"jpg", "png"},
				Quality: 80,
			},
		}
	} else {
		// Default-disabled: Assets struct exists but Images.Enabled
		// stays false. Normalize() will fill in the Images struct so
		// the pipeline can read it without nil-checking.
		cfg.Assets = &config.AssetsConfig{}
	}
	return config.Normalize(cfg)
}

// TestImagePipeline_DisabledIsByteIdentical verifies the spec §9
// acceptance criterion #1: when images are disabled (the v1.6
// default), runImagePipeline must be a no-op and must not touch
// the output directory at all.
func TestImagePipeline_DisabledIsByteIdentical(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, false)

	stats, err := runImagePipeline(imagePosts(root), nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("runImagePipeline: %v", err)
	}
	if stats.Sources != 0 || stats.Variants != 0 || stats.Skipped != 0 || stats.Errors != 0 {
		t.Errorf("disabled run should report zero stats, got %+v", stats)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("disabled run should not write to outputDir, found: %v", names)
	}
}

// TestImagePipeline_EnabledGeneratesPictureTags verifies the spec §9
// acceptance criterion #2: when images are enabled, the manifest
// is written, the variants are on disk, and PostprocessHTML rewrites
// <img src> to <picture> + <source> using the manifest. The test
// runs runImagePipeline + PostprocessHTML directly (no full
// Generate) so it does not need a templates directory.
func TestImagePipeline_EnabledGeneratesPictureTags(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, true)

	// 1) Run the image pipeline.
	if _, err := runImagePipeline(imagePosts(root), nil, cfg, outputDir); err != nil {
		t.Fatalf("runImagePipeline: %v", err)
	}

	// 2) Confirm 2 source × 3 widths × 2 formats = 12 variants.
	entries, err := os.ReadDir(filepath.Join(outputDir, "img"))
	if err != nil {
		t.Fatalf("read img dir: %v", err)
	}
	if len(entries) != 12 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 12 variant files, got %d: %v", len(entries), names)
	}

	// 3) Write a renderable HTML that references one of the
	//    sources, then run PostprocessHTML with the manifest as
	//    input. The postprocess step is what turns the rendered
	//    <img> into a <picture>; without it the test would only
	//    exercise the disk side of the pipeline.
	htmlPath := filepath.Join(outputDir, "post", "index.html")
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0755); err != nil {
		t.Fatal(err)
	}
	htmlIn := []byte(`<p><img src="/img/inline.jpg" alt="Inline"></p>`)
	if err := os.WriteFile(htmlPath, htmlIn, 0644); err != nil {
		t.Fatal(err)
	}
	imageSources := loadImageManifestForPostprocess(outputDir)
	if imageSources == nil {
		t.Fatal("postprocess helper returned nil; manifest is missing or unreadable")
	}
	stats, err := PostprocessHTML(PostprocessOptions{
		OutputDir:    outputDir,
		ImageSources: imageSources,
	})
	if err != nil {
		t.Fatalf("PostprocessHTML: %v", err)
	}
	if stats.ImageReferencesFound != 1 || stats.ImageReferencesRewritten != 1 {
		t.Errorf("postprocess stats = %+v, want 1/1", stats)
	}

	// 4) Re-read the HTML and assert the rewrite produced a
	//    <picture> block with the expected <source> tags.
	htmlOut, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(htmlOut)
	if !strings.Contains(out, "<picture>") {
		t.Errorf("expected <picture> block, got:\n%s", out)
	}
	if !strings.Contains(out, `<source type="image/jpg"`) {
		t.Errorf("expected <source type=image/jpg>, got:\n%s", out)
	}
	if !strings.Contains(out, `<source type="image/png"`) {
		t.Errorf("expected <source type=image/png>, got:\n%s", out)
	}
	if !strings.Contains(out, `srcset="/img/inline-480w.jpg 480w, /img/inline-800w.jpg 800w, /img/inline-1200w.jpg 1200w"`) {
		t.Errorf("expected jpg srcset on the fallback <img>, got:\n%s", out)
	}
}

// TestImagePipeline_FrontMatterCover verifies the spec §9
// acceptance criterion #2 (front matter side): the cover field in
// a post's front matter is picked up by the image pipeline and
// turned into variants that show up in the manifest and on disk.
func TestImagePipeline_FrontMatterCover(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, true)

	if _, err := runImagePipeline(imagePosts(root), nil, cfg, outputDir); err != nil {
		t.Fatalf("runImagePipeline: %v", err)
	}

	// Cover image must be in the manifest AND have variants on disk.
	manifestPath := filepath.Join(outputDir, ".gobin-images.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !bytes.Contains(manifestData, []byte("/img/cover.jpg")) {
		t.Errorf("manifest should contain cover entry, got: %s", manifestData)
	}
	for _, name := range []string{
		"cover-480w.jpg", "cover-800w.jpg", "cover-1200w.jpg",
		"cover-480w.png", "cover-800w.png", "cover-1200w.png",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, "img", name)); err != nil {
			t.Errorf("missing cover variant %s: %v", name, err)
		}
	}
}

// TestImagePipeline_Incremental verifies the spec §9 acceptance
// criterion #5: a second run with no source changes must skip the
// transform step entirely. We assert via the ImageStats.Skipped
// counter, which is populated by runImagePipeline when (and only
// when) the per-source skip path matches the previous run.
func TestImagePipeline_Incremental(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, true)
	posts := imagePosts(root)

	// First run: cold build. We expect 2 sources, 12 variants, 0
	// skipped, 0 errors.
	stats1, err := runImagePipeline(posts, nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("runImagePipeline #1: %v", err)
	}
	if stats1.Sources != 2 {
		t.Errorf("run #1 sources = %d, want 2", stats1.Sources)
	}
	if stats1.Skipped != 0 {
		t.Errorf("run #1 skipped = %d, want 0 (cold)", stats1.Skipped)
	}
	if stats1.Variants == 0 {
		t.Errorf("run #1 variants = 0, want > 0")
	}

	// Second run: nothing has changed. Every source should be
	// skipped because the source hash and the options hash both
	// match the previous run, and every variant file is still on
	// disk. We use the lower-level runImagePipeline so we can read
	// Skipped directly without going through the full Generate.
	stats2, err := runImagePipeline(posts, nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("runImagePipeline #2: %v", err)
	}
	if stats2.Skipped != stats1.Sources {
		t.Errorf("run #2 skipped = %d, want %d (all sources should be skipped)", stats2.Skipped, stats1.Sources)
	}
	if stats2.Variants != 0 {
		t.Errorf("run #2 variants = %d, want 0 (skip path should not write)", stats2.Variants)
	}
	if stats2.Errors != 0 {
		t.Errorf("run #2 errors = %d, want 0", stats2.Errors)
	}

	// Third run: a variant file is removed. The source hash and
	// options hash still match, but the on-disk check should fail
	// and the source must be re-transformed. We delete one of the
	// cover variants and re-run.
	coverVariant := filepath.Join(outputDir, "img", "cover-480w.jpg")
	if err := os.Remove(coverVariant); err != nil {
		t.Fatalf("remove cover variant: %v", err)
	}
	stats3, err := runImagePipeline(posts, nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("runImagePipeline #3: %v", err)
	}
	if stats3.Skipped != stats1.Sources-1 {
		t.Errorf("run #3 skipped = %d, want %d (only one source re-transformed)", stats3.Skipped, stats1.Sources-1)
	}
	if stats3.Variants == 0 {
		t.Errorf("run #3 variants = 0, want > 0 (missing variant should force re-transform)")
	}
	if _, err := os.Stat(coverVariant); err != nil {
		t.Errorf("expected %s to be rewritten on run #3, got: %v", coverVariant, err)
	}
}

// TestImagePipeline_SourceChangeTriggersRetransform verifies the
// other half of the v1.7.1 incremental skip: a change to the
// source bytes must invalidate the skip for that source. We touch
// the inline image between runs and expect the inline source to
// be re-transformed while the cover source stays skipped.
func TestImagePipeline_SourceChangeTriggersRetransform(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, true)
	posts := imagePosts(root)

	if _, err := runImagePipeline(posts, nil, cfg, outputDir); err != nil {
		t.Fatalf("run #1: %v", err)
	}

	// Change the inline image source bytes. We pick a new size
	// (the gradient is deterministic per (w, h) so writing with
	// the same dimensions would produce a byte-identical file
	// and the test would pass for the wrong reason).
	writeTestJpeg(t, filepath.Join(root, "assets", "img", "inline.jpg"), 1000, 750)

	stats, err := runImagePipeline(posts, nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("run #2: %v", err)
	}
	// Cover source unchanged → skipped. Inline source changed →
	// not skipped (re-transformed). So Skipped should be exactly 1.
	if stats.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (cover unchanged, inline changed)", stats.Skipped)
	}
	if stats.Errors != 0 {
		t.Errorf("Errors = %d, want 0", stats.Errors)
	}
}

// TestImagePipeline_PerSourceFailureDoesNotAbort verifies the spec
// §9 acceptance criterion #6: a per-source transform failure must
// log a warning and continue the build, never abort. We point one
// of the refs at a non-existent file and confirm the build still
// produces variants for the other source and the error counter
// is bumped.
func TestImagePipeline_PerSourceFailureDoesNotAbort(t *testing.T) {
	root := t.TempDir()
	makeImagePipelineSite(t, root)
	// Replace inline.jpg with a non-existent file by deleting it.
	if err := os.Remove(filepath.Join(root, "assets", "img", "inline.jpg")); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(root, "public")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := imagePipelineTestConfig(root, true)
	posts := imagePosts(root)

	stats, err := runImagePipeline(posts, nil, cfg, outputDir)
	if err != nil {
		t.Fatalf("runImagePipeline should not error on a single bad source, got: %v", err)
	}
	if stats.Errors == 0 {
		t.Errorf("Errors = 0, want > 0 (bad source should bump the counter)")
	}
	// Cover image should still be transformed.
	if _, err := os.Stat(filepath.Join(outputDir, "img", "cover-800w.jpg")); err != nil {
		t.Errorf("cover variant missing after partial failure: %v", err)
	}
}
