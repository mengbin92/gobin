package imaging

import (
	"bytes"
	"image"
	"io"

	"github.com/HugoSmits86/nativewebp"
)

// newWebPEncoderForTest returns a function that encodes an image to WebP
// using nativewebp, with quality mapped to a compression level. It is a
// test-only helper that lets the test file build WebP fixtures without
// importing the third-party package in imaging_test.go's import block.
func newWebPEncoderForTest() func(io.Writer, image.Image, int) error {
	return func(w io.Writer, img image.Image, quality int) error {
		return nativewebp.Encode(w, img, &nativewebp.Options{
			CompressionLevel: qualityToCompressionLevel(quality),
		})
	}
}

// webpNativeDecode decodes WebP bytes via nativewebp (which wraps
// golang.org/x/image/webp). Test-only helper.
func webpNativeDecode(b []byte) (image.Image, error) {
	return nativewebp.Decode(bytes.NewReader(b))
}
