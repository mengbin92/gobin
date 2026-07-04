package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
)

// StdlibExecutor is the default Executor implementation. It uses only the
// Go standard library (image/jpeg, image/png) and supports:
//
//   - decode: JPEG, PNG
//   - encode: JPEG (quality honored), PNG (quality ignored)
//
// Other formats are passed through at the file-system layer (see the
// Transform passthrough path); StdlibExecutor.Encode returns an error if
// a caller asks it to actively encode an unsupported format. This keeps
// the contract honest: an unsupported encode is a real failure, not a
// silent no-op.
//
// StdlibExecutor is stateless and safe for concurrent use.
type StdlibExecutor struct{}

// NewStdlibExecutor returns a StdlibExecutor.
func NewStdlibExecutor() *StdlibExecutor { return &StdlibExecutor{} }

// Decode implements Executor.
func (StdlibExecutor) Decode(src []byte) (image.Image, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("imaging: empty source")
	}
	if img, err := jpeg.Decode(bytes.NewReader(src)); err == nil {
		return img, nil
	}
	if img, err := png.Decode(bytes.NewReader(src)); err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("imaging: stdlib executor cannot decode source (only jpeg/png)")
}

// Encode implements Executor.
func (StdlibExecutor) Encode(dst io.Writer, img image.Image, ext string, quality int) error {
	switch normalizeExt(ext) {
	case "jpg", "jpeg":
		q := quality
		if q < 1 || q > 100 {
			q = 75
		}
		return jpeg.Encode(dst, img, &jpeg.Options{Quality: q})
	case "png":
		enc := &png.Encoder{CompressionLevel: png.DefaultCompression}
		return enc.Encode(dst, img)
	default:
		return fmt.Errorf("imaging: stdlib executor cannot encode format %q (only jpg/png)", strings.TrimPrefix(ext, "."))
	}
}
