// Package imaging generates multi-size, multi-format image variants for a
// static site. It is the v1.7 entry point that powers the assets.images
// pipeline: Markdown ![]() and front matter cover/thumbnail references are
// collected at build time, fed through Transform, and the resulting variants
// are written into publishDir. The postprocess step then rewrites
// <img src="..."> in rendered HTML to <picture><source>...<img srcset>.
//
// The package is decoupled from any specific image library: the public API
// takes raw bytes and an explicit Executor. The default StdlibExecutor
// uses only the Go standard library (image/jpeg, image/png,
// image/draw), so the package builds with no third-party
// image-encoding dependencies. A future disintegration/imaging or
// bimg-based executor can be added without touching the call sites.
//
// Formats supported by the stdlib executor:
//   - "jpg" / "jpeg": re-encoded JPEG, quality parameter honored
//   - "png": re-encoded PNG, quality parameter ignored
//   - "webp", "avif", anything else: passthrough (no re-encode) — the
//     source bytes are emitted under the target filename so the build
//     still produces the expected output paths and HTML rewrite stays
//     correct. A future executor that supports these formats can replace
//     this behavior transparently.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Executor performs the actual decode/resize/encode for a single source
// image. Implementations are stateless and safe for concurrent use.
type Executor interface {
	// Decode parses src and returns its bounds. It is called once per source
	// image, before any variants are produced.
	Decode(src []byte) (img image.Image, err error)
	// Encode writes a single variant. The output format is identified by
	// ext (".jpg", ".png", ".webp", ...). The width parameter is the
	// target pixel width; the executor decides the matching height to
	// preserve aspect ratio.
	Encode(dst io.Writer, img image.Image, ext string, quality int) error
}

// TransformOptions controls a single source image's transformation.
type TransformOptions struct {
	// Widths is the list of output widths. An entry of 0 means "use the
	// source width unchanged". Entries must be unique and positive. The
	// executor is free to drop widths that exceed the source image's width
	// (upscaling rarely helps and inflates payload).
	Widths []int
	// Formats is the list of output formats. Entries are matched against
	// the source extension to decide whether the variant needs an actual
	// re-encode or can passthrough. An empty list is treated as
	// {"<source format>"} (one variant, same format, original size).
	Formats []string
	// Quality is the encoding quality in the [1, 100] range. It is only
	// honored for JPEG today; PNG ignores it. A non-positive value falls
	// back to a package default (75 for JPEG).
	Quality int
}

// Variant is one output file produced by Transform.
type Variant struct {
	// OutputName is the basename of the file, e.g. "cover-800w.jpg". It
	// has no path component and no leading slash. Callers combine it with
	// their preferred output directory.
	OutputName string
	Width      int
	Format     string // canonical: "jpg" / "png" / "webp" / ...
	Bytes      []byte
}

// Transform runs the full decode/resize/encode pipeline for one source
// image and returns every requested variant. The output is deterministic
// for a given (src, opts, executor) tuple so callers can compare against
// the on-disk manifest to decide whether the variants are stale.
//
// Variants are returned in a stable order: by Format alphabetically, then
// by Width ascending, then by OutputName. This matches the iteration
// order callers use when emitting <source> tags so diffing is stable.
func Transform(src []byte, sourceExt string, opts TransformOptions, exec Executor) ([]Variant, error) {
	if exec == nil {
		return nil, errors.New("imaging: nil executor")
	}
	if len(src) == 0 {
		return nil, errors.New("imaging: empty source")
	}
	img, err := exec.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("imaging: decode: %w", err)
	}

	widths := normalizeWidths(opts.Widths, img.Bounds().Dx())
	if len(widths) == 0 {
		// No widths requested and source has no width: emit one passthrough
		// variant of the source format so callers always get at least one
		// output to wire up in the HTML rewrite.
		widths = []int{img.Bounds().Dx()}
	}
	formats := normalizeFormats(opts.Formats, sourceExt)
	if len(formats) == 0 {
		formats = []string{normalizeExt(sourceExt)}
	}
	quality := opts.Quality
	if quality <= 0 {
		quality = 75
	}

	const base = "image"
	srcFormat := normalizeExt(sourceExt)

	var variants []Variant
	for _, f := range formats {
		for _, w := range widths {
			resized := resize(img, w)
			outName := buildOutputName(base, w, f)

			if normalizeExt(f) == srcFormat &&
				resized.Bounds().Dx() == img.Bounds().Dx() &&
				resized.Bounds().Dy() == img.Bounds().Dy() {
				// No-op variant: same format, same dimensions. Passthrough
				// the source bytes so we don't pay re-encode cost and the
				// output is bit-identical to the input.
				variants = append(variants, Variant{
					OutputName: outName,
					Width:      resized.Bounds().Dx(),
					Format:     normalizeExt(f),
					Bytes:      append([]byte(nil), src...),
				})
				continue
			}
			var buf bytes.Buffer
			if err := exec.Encode(&buf, resized, "."+normalizeExt(f), quality); err != nil {
				return nil, fmt.Errorf("imaging: encode %s: %w", outName, err)
			}
			variants = append(variants, Variant{
				OutputName: outName,
				Width:      resized.Bounds().Dx(),
				Format:     normalizeExt(f),
				Bytes:      buf.Bytes(),
			})
		}
	}

	sort.SliceStable(variants, func(i, j int) bool {
		if variants[i].Format != variants[j].Format {
			return variants[i].Format < variants[j].Format
		}
		if variants[i].Width != variants[j].Width {
			return variants[i].Width < variants[j].Width
		}
		return variants[i].OutputName < variants[j].OutputName
	})
	return variants, nil
}

// resize returns a copy of src scaled to width w, preserving aspect ratio.
// A width of 0 or greater than the source width returns the source
// unchanged. Internally it delegates to boxScale (a box/area-average
// filter implemented in resize.go) so the package stays dependency-free;
// a future Executor can swap in a higher-quality scaler without touching
// Transform or its callers.
func resize(src image.Image, w int) image.Image {
	srcW := src.Bounds().Dx()
	if w <= 0 || w >= srcW {
		return src
	}
	return boxScale(src, w, 0)
}

// normalizeWidths returns a sorted, deduplicated, source-bounded width
// list. A nil or empty input falls back to the source width so callers
// always get a sensible default.
func normalizeWidths(widths []int, sourceWidth int) []int {
	if len(widths) == 0 {
		return []int{sourceWidth}
	}
	seen := make(map[int]struct{}, len(widths))
	out := make([]int, 0, len(widths))
	for _, w := range widths {
		if w <= 0 {
			continue
		}
		if w > sourceWidth {
			// Skip upscaling; the original is already available via the
			// passthrough variant path.
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	sort.Ints(out)
	return out
}

// normalizeFormats returns the canonical format list for opts.Formats,
// falling back to the source format when the input is empty. Unknown
// formats are passed through (the stdlib executor will treat them as
// passthrough; future executors can recognize and re-encode them).
func normalizeFormats(formats []string, sourceExt string) []string {
	if len(formats) == 0 {
		return []string{normalizeExt(sourceExt)}
	}
	seen := make(map[string]struct{}, len(formats))
	out := make([]string, 0, len(formats))
	for _, f := range formats {
		c := normalizeExt(f)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func normalizeExt(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, ".")
	if s == "jpeg" {
		return "jpg"
	}
	return s
}

// buildOutputName composes a stable output filename. Convention: <base>-<width>w.<format>.
// Examples: image-800w.jpg, image-1200w.png.
func buildOutputName(base string, width int, format string) string {
	return fmt.Sprintf("%s-%dw.%s", base, width, format)
}

// FileBase returns the basename (no extension) of path. It is exported as
// a convenience for callers composing per-source output names.
func FileBase(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// JoinOutputPath joins an output directory with a variant's name.
func JoinOutputPath(dir, name string) string {
	return filepath.Join(dir, name)
}
