package imaging

import (
	"image"
	"image/color"
)

// boxScale returns a copy of src scaled to (dstW, dstH) using a box (area
// average) filter. It preserves aspect ratio when one of dstW/dstH is 0
// by computing it from the source dimensions.
//
// A box filter is the right choice for downscaling: it averages the source
// pixels covered by each output pixel, which is the mathematically
// correct low-pass filter for arbitrary scale changes. It is not as crisp
// as a Lanczos or Catmull-Rom filter for upscaling, but image optimization
// is overwhelmingly a downscale workload and the quality is more than
// good enough for the responsive-image use case.
//
// boxScale depends only on the Go standard library. A future Executor
// can swap in a higher-quality filter (CatmullRom, Lanczos) without
// touching Transform or its callers.
func boxScale(src image.Image, dstW, dstH int) image.Image {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if srcW == 0 || srcH == 0 {
		return src
	}
	if dstW == 0 && dstH == 0 {
		return src
	}
	if dstW == 0 {
		dstW = srcW * dstH / srcH
		if dstW < 1 {
			dstW = 1
		}
	}
	if dstH == 0 {
		dstH = srcH * dstW / srcW
		if dstH < 1 {
			dstH = 1
		}
	}
	if dstW >= srcW && dstH >= srcH {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		y0 := y * srcH / dstH
		y1 := (y + 1) * srcH / dstH
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dstW; x++ {
			x0 := x * srcW / dstW
			x1 := (x + 1) * srcW / dstW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, a uint64
			var n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					b += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n >> 8),
				G: uint8(g / n >> 8),
				B: uint8(b / n >> 8),
				A: uint8(a / n >> 8),
			})
		}
	}
	return dst
}
