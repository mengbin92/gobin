// Package imaging — webp.go provides a real WebP encode/decode backend
// (v1.7.2). It wraps github.com/HugoSmits86/nativewebp, a pure-Go
// (zero-cgo) WebP library: VP8L lossless encode + x/image/webp decode.
//
// Before v1.7.2 the stdlib executor treated "webp" as an unsupported
// format and Transform returned an error (or passthrough at the same
// format+size) whenever a caller requested WebP variants. With this
// file the WebPExecutor can decode .webp sources and re-encode any
// image.Image to WebP, so config.assets.images.formats: ["webp"]
// produces real, browser-loadable WebP bytes.
//
// Encoding is VP8L (lossless). The Quality option from TransformOptions
// is mapped to a CompressionLevel effort trade-off because VP8L has no
// lossy quality axis: a higher quality value selects a higher effort
// level (better ratio, more CPU). This keeps the single Quality knob
// meaningful across formats.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/HugoSmits86/nativewebp"
)

// WebPExecutor is a real WebP encode/decode backend built on
// nativewebp (pure Go, no cgo). It supports:
//
//   - decode: JPEG, PNG, WebP (JPEG/PNG via the stdlib path shared with
//     StdlibExecutor; WebP via nativewebp.Decode, which wraps
//     golang.org/x/image/webp and handles both lossy and lossless).
//   - encode: JPEG (quality), PNG, WebP (VP8L lossless; quality mapped
//     to compression effort).
//
// WebPExecutor is stateless and safe for concurrent use.
type WebPExecutor struct {
	StdlibExecutor
}

// NewWebPExecutor returns a WebPExecutor.
func NewWebPExecutor() *WebPExecutor { return &WebPExecutor{} }

// Decode implements Executor. It first tries the stdlib JPEG/PNG path
// (fast path for the overwhelmingly common blog-image sources) and falls
// back to nativewebp.Decode for .webp sources.
func (WebPExecutor) Decode(src []byte) (image.Image, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("imaging: empty source")
	}
	if img, err := jpeg.Decode(bytes.NewReader(src)); err == nil {
		return img, nil
	}
	if img, err := png.Decode(bytes.NewReader(src)); err == nil {
		return img, nil
	}
	if img, err := nativewebp.Decode(bytes.NewReader(src)); err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("imaging: webp executor cannot decode source (jpeg/png/webp)")
}

// Encode implements Executor. For "webp" it delegates to
// nativewebp.Encode (VP8L lossless), mapping the caller's Quality to a
// CompressionLevel. For "jpg"/"png" it reuses the stdlib encoder so a
// single executor can produce a mixed-format srcset (e.g. ["webp","jpg"]
// for progressive enhancement with a JPEG fallback).
func (e WebPExecutor) Encode(dst io.Writer, img image.Image, ext string, quality int) error {
	switch normalizeExt(ext) {
	case "webp":
		level := qualityToCompressionLevel(quality)
		return nativewebp.Encode(dst, img, &nativewebp.Options{
			CompressionLevel: level,
		})
	default:
		return e.StdlibExecutor.Encode(dst, img, ext, quality)
	}
}

// qualityToCompressionLevel maps the [1,100] quality axis used by the
// rest of the pipeline to nativewebp's [0,6] CompressionLevel. A higher
// quality value means "spend more effort for a better ratio", which is
// the closest VP8L-lossless analogue of "higher quality". The mapping is
// clamped and monotonic.
func qualityToCompressionLevel(quality int) nativewebp.CompressionLevel {
	if quality <= 0 {
		quality = 75
	}
	if quality > 100 {
		quality = 100
	}
	// q=1..100 -> level=0..6 (BestSpeed..BestCompression)
	level := (quality - 1) * 6 / 99
	if level < 0 {
		level = 0
	}
	if level > 6 {
		level = 6
	}
	return nativewebp.CompressionLevel(level)
}

// NewDefaultExecutor returns the executor used by the image pipeline when
// no explicit executor is configured. Since v1.7.2 this is a WebPExecutor
// (a superset of StdlibExecutor that also handles real WebP encode/decode),
// so config.assets.images.formats: ["webp"] works out of the box without
// any extra setup. Callers that need the zero-third-party-dependency
// behavior can still construct NewStdlibExecutor() explicitly.
func NewDefaultExecutor() Executor { return NewWebPExecutor() }
