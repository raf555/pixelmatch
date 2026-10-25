package pixelmatch

import (
	"image"

	v7 "github.com/raf555/pixelmatch/v7"
)

// Option configures a [Compare] or [CompareToImage] call. It is the same
// type as [v7.Option], so you can mix options from both packages.
type Option = v7.Option

// WithThreshold calls [v7.WithThreshold].
func WithThreshold(t float64) Option { return v7.WithThreshold(t) }

// WithIncludeAA calls [v7.WithIncludeAA].
func WithIncludeAA(b bool) Option { return v7.WithIncludeAA(b) }

// WithAlpha calls [v7.WithAlpha].
func WithAlpha(a float64) Option { return v7.WithAlpha(a) }

// WithAAColor calls [v7.WithAAColor].
func WithAAColor(r, g, b uint8) Option { return v7.WithAAColor(r, g, b) }

// WithDiffColor calls [v7.WithDiffColor].
func WithDiffColor(r, g, b uint8) Option { return v7.WithDiffColor(r, g, b) }

// WithDiffColorAlt calls [v7.WithDiffColorAlt].
func WithDiffColorAlt(r, g, b uint8) Option { return v7.WithDiffColorAlt(r, g, b) }

// WithDiffMask calls [v7.WithDiffMask].
func WithDiffMask(b bool) Option { return v7.WithDiffMask(b) }

// WithCheckerboard calls [v7.WithCheckerboard].
func WithCheckerboard(b bool) Option { return v7.WithCheckerboard(b) }

// WithOutput calls [v7.WithOutput].
func WithOutput(out *image.NRGBA) Option { return v7.WithOutput(out) }
