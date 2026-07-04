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
