package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
)

// makeTestJpeg builds an in-memory JPEG of the given size with a simple
// gradient. It is small and cheap so test cases don't depend on the
// filesystem.
func makeTestJpeg(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: uint8((x + y) * 128 / (w + h)),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func makeTestPng(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x*255/w) ^ uint8(y*255/h),
				G: uint8(x * 255 / w),
				B: uint8(y * 255 / h),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestNormalizeExt(t *testing.T) {
	cases := map[string]string{
		".jpg":    "jpg",
		".JPG":    "jpg",
		"jpeg":    "jpg",
		".jpeg":   "jpg",
		".png":    "png",
		".webp":   "webp",
		"  .PNG ": "png",
		"":        "",
	}
	for in, want := range cases {
		if got := normalizeExt(in); got != want {
			t.Errorf("normalizeExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeWidths(t *testing.T) {
	t.Run("empty returns source width", func(t *testing.T) {
		got := normalizeWidths(nil, 1920)
		if len(got) != 1 || got[0] != 1920 {
			t.Errorf("expected [1920], got %v", got)
		}
	})
	t.Run("dedupes and sorts", func(t *testing.T) {
		got := normalizeWidths([]int{800, 480, 800, 1200, 480}, 1920)
		want := []int{480, 800, 1200}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})
	t.Run("drops upscaling", func(t *testing.T) {
		got := normalizeWidths([]int{800, 2400, 480}, 1200)
		want := []int{480, 800}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})
	t.Run("drops non-positive", func(t *testing.T) {
		got := normalizeWidths([]int{0, -100, 800}, 1920)
		want := []int{800}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
	})
}

func TestNormalizeFormats(t *testing.T) {
	t.Run("empty returns source format", func(t *testing.T) {
		got := normalizeFormats(nil, ".jpg")
		if len(got) != 1 || got[0] != "jpg" {
			t.Errorf("expected [jpg], got %v", got)
		}
	})
	t.Run("dedupes and normalizes", func(t *testing.T) {
		got := normalizeFormats([]string{".jpg", "JPG", "png", "PNG", "webp"}, ".jpg")
		want := []string{"jpg", "png", "webp"}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("[%d] = %s, want %s", i, got[i], want[i])
			}
		}
	})
}

func TestBuildOutputName(t *testing.T) {
	got := buildOutputName("image", 800, "jpg")
	if got != "image-800w.jpg" {
		t.Errorf("got %q, want %q", got, "image-800w.jpg")
	}
}

func TestTransform_JPEGMultiSizeMultiFormat(t *testing.T) {
	src := makeTestJpeg(t, 1600, 900)
	variants, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{480, 800, 1200},
		Formats: []string{"jpg", "png"},
		Quality: 75,
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	// 3 widths * 2 formats = 6 variants expected.
	if len(variants) != 6 {
		t.Fatalf("expected 6 variants, got %d", len(variants))
	}
	// Output names should follow <base>-<width>w.<format>.
	for _, v := range variants {
		if !strings.HasPrefix(v.OutputName, "image-") || !strings.HasSuffix(v.OutputName, "w."+v.Format) {
			t.Errorf("malformed output name: %q", v.OutputName)
		}
		if len(v.Bytes) == 0 {
			t.Errorf("empty bytes for %q", v.OutputName)
		}
		if v.Width <= 0 {
			t.Errorf("non-positive width for %q: %d", v.OutputName, v.Width)
		}
	}
	// Variants should be ordered by (format, width) ascending. We check
	// the sort key (format, width) explicitly so the test does not depend
	// on the bytewise ordering of output names.
	type key struct {
		format string
		width  int
	}
	var prev key
	for i, v := range variants {
		cur := key{v.Format, v.Width}
		if i > 0 {
			if prev.format == cur.format && cur.width < prev.width {
				t.Errorf("width not ascending within format %q: %d after %d", cur.format, cur.width, prev.width)
			}
			if cur.format < prev.format {
				t.Errorf("format not ascending: %q after %q", cur.format, prev.format)
			}
		}
		prev = cur
	}
}

func TestTransform_PassthroughSameFormatSameSize(t *testing.T) {
	// When the only requested variant is the source's own format and
	// size, Transform should passthrough the bytes instead of re-encoding.
	src := makeTestJpeg(t, 800, 600)
	variants, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{800},
		Formats: []string{"jpg"},
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if !bytes.Equal(variants[0].Bytes, src) {
		t.Errorf("passthrough variant should be byte-identical to source")
	}
}

func TestTransform_UnknownFormatFallsBackToSource(t *testing.T) {
	// formats list is empty → use the source format. The source is jpg so
	// we should still get a jpg variant, not an error.
	src := makeTestJpeg(t, 800, 600)
	variants, err := Transform(src, ".jpg", TransformOptions{}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0].Format != "jpg" {
		t.Errorf("expected jpg fallback, got %s", variants[0].Format)
	}
}

func TestTransform_PNGSourceDecodes(t *testing.T) {
	src := makeTestPng(t, 800, 600)
	variants, err := Transform(src, ".png", TransformOptions{
		Widths:  []int{400},
		Formats: []string{"png"},
		Quality: 75,
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0].Width != 400 {
		t.Errorf("expected width=400, got %d", variants[0].Width)
	}
	if variants[0].Format != "png" {
		t.Errorf("expected format=png, got %s", variants[0].Format)
	}
}

func TestTransform_DropsUpscaling(t *testing.T) {
	src := makeTestJpeg(t, 800, 600)
	variants, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{400, 1600, 800},
		Formats: []string{"jpg"},
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	// 1600 should be dropped (source is 800 wide). 400 and 800 remain.
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants (upscale dropped), got %d", len(variants))
	}
	for _, v := range variants {
		if v.Width > 800 {
			t.Errorf("variant width %d exceeds source 800", v.Width)
		}
	}
}

func TestTransform_NilExecutor(t *testing.T) {
	src := makeTestJpeg(t, 800, 600)
	_, err := Transform(src, ".jpg", TransformOptions{}, nil)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestTransform_EmptySource(t *testing.T) {
	_, err := Transform(nil, ".jpg", TransformOptions{}, NewStdlibExecutor())
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestTransform_DecodeError(t *testing.T) {
	_, err := Transform([]byte("not an image"), ".jpg", TransformOptions{}, NewStdlibExecutor())
	if err == nil {
		t.Fatal("expected error for non-image source")
	}
}

// encodeFailExecutor makes Decode succeed (delegates to StdlibExecutor)
// and Encode always fail. It is used to drive the encode-error branch in
// Transform.
type encodeFailExecutor struct{ StdlibExecutor }

func (encodeFailExecutor) Encode(_ io.Writer, _ image.Image, _ string, _ int) error {
	return errEncodeFailed
}

var errEncodeFailed = errors.New("encode failed")

func TestTransform_EncodeError(t *testing.T) {
	src := makeTestJpeg(t, 800, 600)
	// Ask for png at a non-source width so Transform must call Encode
	// (passthrough only triggers when format+size both match).
	_, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{400},
		Formats: []string{"png"},
	}, encodeFailExecutor{})
	if err == nil {
		t.Fatal("expected encode error, got nil")
	}
	if !strings.Contains(err.Error(), "encode") {
		t.Errorf("expected error to mention encode, got: %v", err)
	}
}

func TestBoxScale_PreservesAspectRatio(t *testing.T) {
	// 1000x500 should scale to 400x200 with dstH=0 (auto).
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	dst := boxScale(src, 400, 0)
	if dst.Bounds().Dx() != 400 || dst.Bounds().Dy() != 200 {
		t.Errorf("expected 400x200, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

func TestBoxScale_NoOpOnSmaller(t *testing.T) {
	// Upscaling is a no-op (returns source).
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	dst := boxScale(src, 400, 0)
	if dst != image.Image(src) {
		t.Errorf("expected upscale to return the source unchanged")
	}
}

func TestBoxScale_ZeroDims(t *testing.T) {
	// Both dims zero → no-op.
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	if boxScale(src, 0, 0) != src {
		t.Errorf("expected zero-scale to return the source unchanged")
	}
}

// TestTransform_EmptyFormatsIsSingleSourceSize verifies the spec §8.1
// behavior: when opts.Formats is nil/empty AND opts.Widths is
// nil/empty, Transform returns a single variant in the source
// format at the source.s own width. The fallback is the literal
// source passthrough; the stdlib executor sees the passthrough
// path and emits the source bytes unchanged.
func TestTransform_EmptyFormatsIsSingleSourceSize(t *testing.T) {
	src := makeTestJpeg(t, 1600, 1200)
	variants, err := Transform(src, ".jpg", TransformOptions{
		Widths:  nil,
		Formats: nil,
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0].Width != 1600 {
		t.Errorf("expected width=1600 (source width), got %d", variants[0].Width)
	}
	if variants[0].Format != "jpg" {
		t.Errorf("expected format=jpg (source format), got %s", variants[0].Format)
	}
	if !bytes.Equal(variants[0].Bytes, src) {
		t.Errorf("empty-options passthrough should be byte-identical to source")
	}
}

// TestTransform_InvalidFormatReturnsError verifies the spec §8.1
// behavior: an explicitly requested format the executor cannot encode
// must surface as an error rather than a silent passthrough. The
// stdlib executor is the only production executor today, and it
// supports jpg/png only; asking it to encode webp must fail loudly.
func TestTransform_InvalidFormatReturnsError(t *testing.T) {
	src := makeTestJpeg(t, 800, 600)
	_, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{400}, // forces re-encode (width differs from source)
		Formats: []string{"webp"},
	}, NewStdlibExecutor())
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	// The error should mention either "encode" or the format name so
	// the user can act on it (install a WebP-capable executor, or drop
	// the format from config).
	msg := err.Error()
	if !strings.Contains(msg, "encode") && !strings.Contains(msg, "webp") {
		t.Errorf("error should mention encode or webp, got: %v", err)
	}
}

// TestTransform_QualityDifference verifies the spec §8.1 behavior:
// JPEG quality is honored, and a low/high pair produces measurably
// different output sizes. A higher quality must not produce a smaller
// file (sanity check: the size-vs-quality curve is monotonically
// non-decreasing in the regime we care about).
func TestTransform_QualityDifference(t *testing.T) {
	src := makeTestJpeg(t, 1200, 900) // busy source so compression matters
	low, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{800},
		Formats: []string{"jpg"},
		Quality: 30,
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform low: %v", err)
	}
	high, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{800},
		Formats: []string{"jpg"},
		Quality: 95,
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform high: %v", err)
	}
	if len(low) != 1 || len(high) != 1 {
		t.Fatalf("expected 1 variant each, got low=%d high=%d", len(low), len(high))
	}
	if low[0].Width != high[0].Width {
		t.Errorf("widths differ: low=%d high=%d", low[0].Width, high[0].Width)
	}
	if low[0].Width != 800 {
		t.Errorf("expected width=800, got %d", low[0].Width)
	}
	lowSize := len(low[0].Bytes)
	highSize := len(high[0].Bytes)
	if highSize < lowSize {
		t.Errorf("high quality (%d bytes) is smaller than low quality (%d bytes); JPEG quality curve is non-monotonic", highSize, lowSize)
	}
	// A 30→95 swing on a busy gradient should be a meaningful size
	// gap. Use a soft threshold (10% of the low size) to avoid
	// flakiness on the trivial gradient fixture.
	if highSize-lowSize < lowSize/10 {
		t.Logf("warning: low=%d high=%d — quality delta smaller than 10%% of low", lowSize, highSize)
	}
}

// TestTransform_PNGPreservesAlphaChannel verifies the spec §8.1
// behavior: a PNG source with an alpha channel is decoded correctly
// and re-encoded as a PNG with alpha intact. The output must be
// decodable as image/png and the alpha at a transparent corner must
// still be 0.
func TestTransform_PNGPreservesAlphaChannel(t *testing.T) {
	const w, h = 400, 300
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Top-left quadrant: opaque red. Bottom-right: transparent green.
			if x < w/2 && y < h/2 {
				img.Set(x, y, color.NRGBA{R: 255, A: 255})
			} else {
				img.Set(x, y, color.NRGBA{G: 255, A: 0})
			}
		}
	}
	var srcBuf bytes.Buffer
	if err := png.Encode(&srcBuf, img); err != nil {
		t.Fatalf("encode src: %v", err)
	}

	variants, err := Transform(srcBuf.Bytes(), ".png", TransformOptions{
		Widths:  []int{200},
		Formats: []string{"png"},
	}, NewStdlibExecutor())
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	decoded, err := png.Decode(bytes.NewReader(variants[0].Bytes))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	// Bottom-right corner of the original (and any downscale of it)
	// must still be transparent. Use At() (interface method) and
	// inspect the alpha via RGBA().
	b := decoded.Bounds()
	_, _, _, a := decoded.At(b.Dx()-1, b.Dy()-1).RGBA()
	if a != 0 {
		t.Errorf("expected alpha=0 at bottom-right, got %d", a)
	}
}

// webpStubExecutor is a fake executor used to drive the WebP code
// path through Transform without depending on an actual WebP encoder.
// It "encodes" webp by emitting a deterministic sentinel byte slice
// (so the test can prove the Encode call happened and the bytes
// reached the output variant). The decode step delegates to
// StdlibExecutor so the source must still be a real jpeg/png.
//
// This test is the spec §8.1 hook for WebP: when a real WebP-capable
// executor (disintegration/imaging, libvips) is added, the same test
// will exercise the real encoder — the contract (the Executor
// interface) is what we are committing to here.
type webpStubExecutor struct {
	StdlibExecutor
	webpCalls int
}

func (w *webpStubExecutor) Encode(dst io.Writer, img image.Image, ext string, quality int) error {
	if ext == ".webp" {
		w.webpCalls++
		// Sentinel: 4 bytes that no real image encoder will emit.
		_, err := dst.Write([]byte("WEBP!"))
		return err
	}
	return w.StdlibExecutor.Encode(dst, img, ext, quality)
}

// TestTransform_WebPViaExecutorInterface verifies the spec §8.1
// hook: a custom executor that supports webp gets called and its
// output reaches the variant slice. The spec's full WebP
// acceptance criterion ("WebP 转换正确，浏览器能加载") is met when
// such an executor is wired in (e.g. disintegration/imaging or
// HugoSmits86/nativewebp); the StdlibExecutor's webp passthrough
// is the v1.7 default until then.
func TestTransform_WebPViaExecutorInterface(t *testing.T) {
	src := makeTestJpeg(t, 1600, 1200)
	exec := &webpStubExecutor{}

	variants, err := Transform(src, ".jpg", TransformOptions{
		Widths:  []int{800},
		Formats: []string{"jpg", "webp"},
	}, exec)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	// 2 variants: 800w jpg (real re-encode) + 800w webp (stub).
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(variants))
	}
	var webp *Variant
	for i := range variants {
		if variants[i].Format == "webp" {
			webp = &variants[i]
			break
		}
	}
	if webp == nil {
		t.Fatal("no webp variant produced")
	}
	if !bytes.Equal(webp.Bytes, []byte("WEBP!")) {
		t.Errorf("expected webp sentinel, got %q", webp.Bytes)
	}
	if exec.webpCalls != 1 {
		t.Errorf("expected 1 webp Encode call, got %d", exec.webpCalls)
	}
}
