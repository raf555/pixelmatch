package pixelmatch

import (
	"errors"
	"fmt"
	"image"
	"image/draw"

	"github.com/raf555/pixelmatch/internal/pixelmatch"
)

// Option configures a Compare or CompareToImage call. Options compose: pass
// any number to a single call. Unspecified options take the package
// defaults (see DefaultOptions for the underlying values).
//
// Example:
//
//	out := image.NewNRGBA(image.Rect(0, 0, w, h))
//	n, err := pixelmatch.Compare(a, b,
//	    pixelmatch.WithThreshold(0.05),
//	    pixelmatch.WithDiffColor(255, 0, 255),
//	    pixelmatch.WithOutput(out),
//	)
type Option func(*config)

// config bundles the algorithm options and the (optional) output
// destination for one Compare call. It's an internal type so the public
// Option function-type can carry the output target alongside the
// per-pixel parameters without exposing an *image.NRGBA field on the
// byte-API Options struct (which has no business knowing about
// image.Image).
type config struct {
	opts   pixelmatch.Options
	output *image.NRGBA
}

func defaultConfig() config {
	return config{opts: pixelmatch.DefaultOptions()}
}

// WithThreshold sets the matching threshold (0..1). Smaller is more
// sensitive. Default 0.1.
func WithThreshold(t float64) Option {
	return func(c *config) { c.opts.Threshold = t }
}

// WithIncludeAA, if true, disables anti-aliased pixel detection so AA
// pixels are counted as real differences. Default false.
func WithIncludeAA(b bool) Option {
	return func(c *config) { c.opts.IncludeAA = b }
}

// WithAlpha sets the opacity (0..1) of the original image in the diff
// output. 0 = pure white, 1 = original brightness. Default 0.1.
func WithAlpha(a float64) Option {
	return func(c *config) { c.opts.Alpha = a }
}

// WithAAColor sets the RGB color used for anti-aliased pixels in the diff
// output. Default 255, 255, 0 (yellow).
func WithAAColor(r, g, b uint8) Option {
	return func(c *config) { c.opts.AAColor = [3]uint8{r, g, b} }
}

// WithDiffColor sets the RGB color used for differing pixels in the diff
// output. Default 255, 0, 0 (red).
func WithDiffColor(r, g, b uint8) Option {
	return func(c *config) { c.opts.DiffColor = [3]uint8{r, g, b} }
}

// WithDiffColorAlt sets an alternative RGB color used for pixels in img2
// that are darker than img1, letting you distinguish "added" from
// "removed" content. By default no alt color is used.
func WithDiffColorAlt(r, g, b uint8) Option {
	return func(c *config) {
		c.opts.DiffColorAlt = [3]uint8{r, g, b}
		c.opts.HasDiffColorAlt = true
	}
}

// WithDiffMask, if true, draws the diff over a transparent background
// instead of over the (faded) original image. Anti-aliased pixels are not
// drawn in mask mode. Default false.
func WithDiffMask(b bool) Option {
	return func(c *config) { c.opts.DiffMask = b }
}

// WithCheckerboard controls whether semi-transparent pixels are blended
// against a checkerboard pattern (true) or plain white (false). Default
// true; pre-v7 pixelmatch behavior is false.
func WithCheckerboard(b bool) Option {
	return func(c *config) { c.opts.Checkerboard = b }
}

// WithOutput sets the destination image to which the visual diff will be
// written. Without this option, Compare only counts mismatched pixels and
// does not produce a diff image (which is faster).
//
// The output's dimensions must match img1 and img2. The most efficient
// case is an *image.NRGBA with a tight stride and zero origin, which is
// written directly; other layouts go through an internal buffer.
//
// To get a freshly allocated diff image without managing the buffer
// yourself, use CompareToImage instead.
func WithOutput(out *image.NRGBA) Option {
	return func(c *config) { c.output = out }
}

// Compare compares two images and returns the number of mismatched
// pixels. By default no diff image is produced; pass WithOutput to write
// a visual diff into a buffer of your choice.
//
// Compare accepts any image.Image as input. *image.NRGBA goes through a
// zero-copy fast path; *image.RGBA un-premultiplies on the fly; other
// types are converted via draw.Draw. For maximum performance, pass
// *image.NRGBA (this is the layout pixelmatch operates on natively:
// straight, non-premultiplied RGBA).
//
// Compare returns an error if the input images differ in size, if a
// WithOutput target has the wrong size, or if any image is nil.
func Compare(img1, img2 image.Image, opts ...Option) (int, error) {
	if img1 == nil || img2 == nil {
		return 0, errors.New("pixelmatch: img1 and img2 must not be nil")
	}

	b1 := img1.Bounds()
	b2 := img2.Bounds()
	w, h := b1.Dx(), b1.Dy()
	if w != b2.Dx() || h != b2.Dy() {
		return 0, fmt.Errorf("pixelmatch: image dimensions do not match: %dx%d vs %dx%d",
			w, h, b2.Dx(), b2.Dy())
	}
	if w == 0 || h == 0 {
		return 0, errors.New("pixelmatch: image dimensions must be positive")
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.output != nil {
		ob := cfg.output.Bounds()
		if ob.Dx() != w || ob.Dy() != h {
			return 0, fmt.Errorf("pixelmatch: output dimensions do not match: %dx%d vs %dx%d",
				ob.Dx(), ob.Dy(), w, h)
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
// Passing WithOutput here is harmless but redundant — CompareToImage
// always returns its own allocated image.
func CompareToImage(img1, img2 image.Image, opts ...Option) (*image.NRGBA, int, error) {
	if img1 == nil {
		return nil, 0, errors.New("pixelmatch: img1 must not be nil")
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
		for y := 0; y < h; y++ {
			srcRow := src.PixOffset(b.Min.X, b.Min.Y+y)
			copy(buf[y*4*w:(y+1)*4*w], src.Pix[srcRow:srcRow+4*w])
		}
		return buf

	case *image.RGBA:
		// Premultiplied → straight. Reverse the (c*a/255) operation.
		buf := make([]byte, 4*w*h)
		for y := 0; y < h; y++ {
			srcRow := src.PixOffset(b.Min.X, b.Min.Y+y)
			dstRow := y * 4 * w
			for x := 0; x < w; x++ {
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
	for y := 0; y < h; y++ {
		dstRow := dst.PixOffset(dst.Rect.Min.X, dst.Rect.Min.Y+y)
		copy(dst.Pix[dstRow:dstRow+4*w], src[y*4*w:(y+1)*4*w])
	}
}

// Compile-time assertion that *image.NRGBA satisfies draw.Image.
var _ draw.Image = (*image.NRGBA)(nil)
