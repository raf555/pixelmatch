package pixelmatch

import (
	"image"

	"github.com/raf555/pixelmatch/internal/pixelmatch"
)

// Option configures a Compare or CompareToImage call.
// Unspecified options take the package defaults
// (see [DefaultOptions] for the underlying values).
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

type config struct {
	opts   pixelmatch.Options
	output *image.NRGBA
}

func defaultConfig() config {
	return config{
		opts: pixelmatch.DefaultOptions(),
	}
}

// WithThreshold sets the matching threshold (0..1). Smaller is more
// sensitive. Default 0.1.
func WithThreshold(t float64) Option {
	return func(c *config) {
		c.opts.Threshold = t
	}
}

// WithIncludeAA, if true, disables anti-aliased pixel detection so AA
// pixels are counted as real differences. Default false.
func WithIncludeAA(b bool) Option {
	return func(c *config) {
		c.opts.IncludeAA = b
	}
}

// WithAlpha sets the opacity (0..1) of the original image in the diff
// output. 0 = pure white, 1 = original brightness. Default 0.1.
func WithAlpha(a float64) Option {
	return func(c *config) {
		c.opts.Alpha = a
	}
}

// WithAAColor sets the RGB color used for anti-aliased pixels in the diff
// output. Default 255, 255, 0 (yellow).
func WithAAColor(r, g, b uint8) Option {
	return func(c *config) {
		c.opts.AAColor = [3]uint8{r, g, b}
	}
}

// WithDiffColor sets the RGB color used for differing pixels in the diff
// output. Default 255, 0, 0 (red).
func WithDiffColor(r, g, b uint8) Option {
	return func(c *config) {
		c.opts.DiffColor = [3]uint8{r, g, b}
	}
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
	return func(c *config) {
		c.opts.DiffMask = b
	}
}

// WithCheckerboard controls whether semi-transparent pixels are blended
// against a checkerboard pattern (true) or plain white (false). Default
// true.
func WithCheckerboard(b bool) Option {
	return func(c *config) {
		c.opts.Checkerboard = b
	}
}

// WithOutput sets the destination image to which the visual diff will be
// written. Without this option, Compare only counts mismatched pixels and
// does not produce a diff image (which is faster).
//
// The output's dimensions must match img1 and img2. The most efficient
// case is an *[image.NRGBA] with a tight stride and zero origin, which is
// written directly; other layouts go through an internal buffer.
//
// To get a freshly allocated diff image without managing the buffer
// yourself, use CompareToImage instead.
func WithOutput(out *image.NRGBA) Option {
	return func(c *config) {
		c.output = out
	}
}
