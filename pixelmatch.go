// Package pixelmatch is a native Go port of mapbox/pixelmatch:
// the smallest, simplest and fastest pixel-level image comparison library.
//
// It compares two raw RGBA images (4 bytes per pixel: R, G, B, A) of equal
// dimensions and optionally writes a diff image to an output buffer. It
// returns the number of mismatched pixels.
//
// Original JS implementation: https://github.com/mapbox/pixelmatch.
//
// Algorithm references:
//   - "Measuring perceived color difference using YIQ NTSC transmission color
//     space in mobile applications" (Kotsarenko & Ramos, 2010).
//   - "Anti-aliased Pixel and Intensity Slope Detector" (Vyšniauskas, 2009).
package pixelmatch

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/raf555/pixelmatch/internal/pixelmatch"
)

// Compare compares two images and returns the number of mismatched
// pixels. By default no diff image is produced; pass [WithOutput] to write
// a visual diff into a buffer of your choice.
//
// Compare accepts any [image.Image] as input. *[image.NRGBA] goes through a
// zero-copy fast path; *[image.RGBA] un-premultiplies on the fly; other
// types are converted via [draw.Draw]. For maximum performance, pass
// *[image.NRGBA] (this is the layout pixelmatch operates on natively:
// straight, non-premultiplied RGBA).
//
// Compare returns an error if the input images differ in size, if a
// [WithOutput] target has the wrong size, or if any image is nil.
func Compare(img1, img2 image.Image, opts ...Option) (int, error) {
	if img1 == nil || img2 == nil {
		return 0, ErrNilImage
	}

	b1 := img1.Bounds()
	b2 := img2.Bounds()
	w, h := b1.Dx(), b1.Dy()
	if w != b2.Dx() || h != b2.Dy() {
		return 0, fmt.Errorf("%w: %dx%d vs %dx%d", ErrDimensionMismatch, w, h, b2.Dx(), b2.Dy())
	}
	if w == 0 || h == 0 {
		return 0, ErrInvalidDimensions
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.output != nil {
		ob := cfg.output.Bounds()
		if ob.Dx() != w || ob.Dy() != h {
			return 0, fmt.Errorf("%w: %dx%d vs %dx%d", ErrOutputDimensionMismatch, ob.Dx(), ob.Dy(), w, h)
		}
	}

	// Materialize inputs as straight-RGBA byte buffers. Fast paths avoid
	// per-pixel conversion entirely.
	pix1 := asStraightRGBA(img1)
	pix2 := asStraightRGBA(img2)

	// Decide what raw output buffer to write into. If the caller's
	// output is an NRGBA with stride == 4*w AND origin at (0,0), we can
	// write straight into its Pix slice. Otherwise we write into a temp
	// buffer and copy back at the end.
	var outBuf []byte
	var temp []byte
	if cfg.output != nil {
		if cfg.output.Stride == 4*w && cfg.output.Rect.Min == (image.Point{}) {
			outBuf = cfg.output.Pix[:4*w*h]
		} else {
			temp = make([]byte, 4*w*h)
			outBuf = temp
		}
	}

	n, err := pixelmatch.Match(pix1, pix2, outBuf, w, h, &cfg.opts)
	if err != nil {
		return 0, err
	}

	if temp != nil {
		copyPixToNRGBA(temp, cfg.output, w, h)
	}
	return n, nil
}

// CompareToImage compares two images and returns a freshly allocated
// diff image alongside the mismatched-pixel count. It's a convenience
// wrapper for callers who want a diff without managing the output
// buffer themselves.
//
// Equivalent to:
//
//	out := image.NewNRGBA(image.Rect(0, 0, w, h))
//	n, err := Compare(img1, img2, append(opts, WithOutput(out))...)
//
// Passing [WithOutput] here is harmless but redundant — [CompareToImage]
// always returns its own allocated image.
func CompareToImage(img1, img2 image.Image, opts ...Option) (*image.NRGBA, int, error) {
	if img1 == nil {
		return nil, 0, ErrNilImage
	}
	b := img1.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// Force our own output regardless of what the caller passed.
	n, err := Compare(img1, img2, append(opts, WithOutput(out))...)
	if err != nil {
		return nil, 0, err
	}
	return out, n, nil
}

// asStraightRGBA returns the image's pixels as a tightly-packed
// width*height*4-byte slice in straight (non-premultiplied) R, G, B, A
// order — the layout pixelmatch operates on natively.
//
// Fast paths:
//   - *image.NRGBA with zero origin and stride == 4*w: returns its Pix
//     slice directly (no copy, no allocation).
//   - *image.NRGBA with other layout: re-packs into a fresh buffer.
//   - *image.RGBA: un-premultiplies into a fresh buffer.
//   - everything else: draw.Draw into a temporary NRGBA, which handles
//     every color model in the stdlib correctly.
//
// Callers must treat the returned buffer as read-only — it may alias the
// source image.
func asStraightRGBA(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	switch src := img.(type) {
	case *image.NRGBA:
		// Tight, zero-origin NRGBA: zero-copy fast path.
		if src.Stride == 4*w && src.Rect.Min == b.Min {
			return src.Pix[:4*w*h]
		}
		// Re-pack a non-tight NRGBA.
		buf := make([]byte, 4*w*h)
		for y := range h {
			srcRow := src.PixOffset(b.Min.X, b.Min.Y+y)
			copy(buf[y*4*w:(y+1)*4*w], src.Pix[srcRow:srcRow+4*w])
		}
		return buf

	case *image.RGBA:
		// Premultiplied → straight. Reverse the (c*a/255) operation.
		buf := make([]byte, 4*w*h)
		for y := range h {
			srcRow := src.PixOffset(b.Min.X, b.Min.Y+y)
			dstRow := y * 4 * w
			for x := range w {
				si := srcRow + x*4
				di := dstRow + x*4
				r := src.Pix[si]
				g := src.Pix[si+1]
				bb := src.Pix[si+2]
				a := src.Pix[si+3]
				switch a {
				case 0:
					// Fully transparent: leave RGB at zero.
					buf[di+3] = 0
				case 0xff:
					buf[di] = r
					buf[di+1] = g
					buf[di+2] = bb
					buf[di+3] = a
				default:
					af := uint32(a)
					buf[di] = uint8(uint32(r) * 0xff / af)
					buf[di+1] = uint8(uint32(g) * 0xff / af)
					buf[di+2] = uint8(uint32(bb) * 0xff / af)
					buf[di+3] = a
				}
			}
		}
		return buf

	default:
		// Generic path: works for any image.Image — Gray, Paletted,
		// YCbCr, etc. Per-pixel via draw.Draw, which handles every
		// stdlib color model correctly.
		tmp := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(tmp, tmp.Bounds(), img, b.Min, draw.Src)
		return tmp.Pix
	}
}

// copyPixToNRGBA writes a tightly-packed RGBA byte buffer into an
// arbitrary NRGBA image, respecting its stride and origin.
func copyPixToNRGBA(src []byte, dst *image.NRGBA, w, h int) {
	if dst.Stride == 4*w && dst.Rect.Min == (image.Point{}) {
		copy(dst.Pix, src)
		return
	}
	for y := range h {
		dstRow := dst.PixOffset(dst.Rect.Min.X, dst.Rect.Min.Y+y)
		copy(dst.Pix[dstRow:dstRow+4*w], src[y*4*w:(y+1)*4*w])
	}
}
