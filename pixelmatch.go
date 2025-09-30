package pixelmatch

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
)

var ErrImageSizesNotMatch = errors.New("image sizes do not match")

type MatchOptions struct {
	threshold        float64
	includeAA        bool
	alpha            float64
	antiAliasedColor color.RGBA
	diffColor        color.RGBA
	diffColorAlt     *color.RGBA
	diffMask         bool
	writeTo          *image.Image
}

type MatchOptionFn func(*MatchOptions)

func WithThreshold(threshold float64) MatchOptionFn {
	return func(o *MatchOptions) {
		o.threshold = threshold
	}
}

func WithDiffDest(img *image.Image) MatchOptionFn {
	return func(o *MatchOptions) {
		o.writeTo = img
	}
}

func WithAntiAlias(aa bool) MatchOptionFn {
	return func(o *MatchOptions) {
		o.includeAA = aa
	}
}

func WithAlpha(alpha float64) MatchOptionFn {
	return func(o *MatchOptions) {
		o.alpha = alpha
	}
}

func WithAntiAliasedColor(c color.Color) MatchOptionFn {
	return func(o *MatchOptions) {
		o.antiAliasedColor = color.RGBAModel.Convert(c).(color.RGBA)
	}
}

func WithDiffColor(c color.Color) MatchOptionFn {
	return func(o *MatchOptions) {
		o.diffColor = color.RGBAModel.Convert(c).(color.RGBA)
	}
}

func WithDiffColorAlt(c color.Color) MatchOptionFn {
	return func(o *MatchOptions) {
		diffColorAlt := color.RGBAModel.Convert(c).(color.RGBA)
		o.diffColorAlt = &diffColorAlt
	}
}

func WithDiffMask(diffMask bool) MatchOptionFn {
	return func(o *MatchOptions) {
		o.diffMask = diffMask
	}
}

type rgba struct {
	R, G, B, A uint32
}

func MatchPixel(a, b image.Image, opts ...MatchOptionFn) (diff int, err error) {
	options := MatchOptions{
		threshold:        0.1,
		alpha:            0.1,
		antiAliasedColor: color.RGBA{R: 255, G: 255, B: 0, A: 255},
		diffColor:        color.RGBA{R: 255, G: 0, B: 0, A: 255},
	}
	for _, opt := range opts {
		opt(&options)
	}

	if !a.Bounds().Eq(b.Bounds()) {
		return 0, ErrImageSizesNotMatch
	}

	bounds := a.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var out *image.RGBA
	if options.writeTo != nil {
		out = image.NewRGBA(bounds)
		defer func() {
			if err == nil {
				*options.writeTo = out
			}
		}()
	}

	if isIdentical(a, b) {
		if out != nil && !options.diffMask {
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					drawGrayPixel(a, x, y, options.alpha, out)
				}
			}
		}
		return 0, nil
	}

	maxDelta := 35215 * options.threshold * options.threshold
	aaColor := options.antiAliasedColor
	diffColor := options.diffColor
	var diffColorAlt color.RGBA
	if options.diffColorAlt != nil {
		diffColorAlt = *options.diffColorAlt
	} else {
		diffColorAlt = diffColor
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pos := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()

			pixel1 := rgba{R: r1, G: g1, B: b1, A: a1}
			pixel2 := rgba{R: r2, G: g2, B: b2, A: a2}

			delta := colorDelta(&pixel1, &pixel2, pos, false)
			if math.Abs(delta) > maxDelta {
				if !options.includeAA && (isAntiAliased(a, x, y, width, height, b) || isAntiAliased(b, x, y, width, height, a)) {
					if out != nil && !options.diffMask {
						drawPixel(out, x, y, aaColor.R, aaColor.G, aaColor.B)
					}
				} else {
					if out != nil {
						if delta < 0 {
							drawPixel(out, x, y, diffColorAlt.R, diffColorAlt.G, diffColorAlt.B)
						} else {
							drawPixel(out, x, y, diffColor.R, diffColor.G, diffColor.B)
						}
					}
					diff++
				}
			} else if out != nil && !options.diffMask {
				drawGrayPixel(a, x, y, options.alpha, out)
			}
		}
	}

	return diff, nil
}

func drawPixel(out *image.RGBA, x, y int, r, g, b uint8) {
	out.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

func drawGrayPixel(img image.Image, x, y int, alpha float64, out *image.RGBA) {
	r, g, b, a := img.At(x, y).RGBA()
	rf := float64(r>>8) * float64(a>>8) / 255.0
	gf := float64(g>>8) * float64(a>>8) / 255.0
	bf := float64(b>>8) * float64(a>>8) / 255.0
	yVal := rf*0.29889531 + gf*0.58662247 + bf*0.11448223
	val := uint8(255 + (yVal-255)*alpha)
	drawPixel(out, x, y, val, val, val)
}

func checkerboardBackground(pos int) (float64, float64, float64) {
	rb := 48 + 159*(pos%2)
	gb := 48 + 159*((int(float64(pos)/1.618033988749895))%2)
	bb := 48 + 159*((int(float64(pos)/2.618033988749895))%2)
	return float64(rb), float64(gb), float64(bb)
}

func colorDelta(a, b *rgba, pos int, yOnly bool) float64 {
	r1 := float64(a.R >> 8)
	g1 := float64(a.G >> 8)
	b1 := float64(a.B >> 8)
	a1 := float64(a.A >> 8)
	r2 := float64(b.R >> 8)
	g2 := float64(b.G >> 8)
	b2 := float64(b.B >> 8)
	a2 := float64(b.A >> 8)

	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	da := a1 - a2

	if dr == 0 && dg == 0 && db == 0 && da == 0 {
		return 0
	}

	if a1 < 255 || a2 < 255 {
		rb, gb, bb := checkerboardBackground(pos)
		dr = (r1*a1 - r2*a2 - rb*da) / 255.0
		dg = (g1*a1 - g2*a2 - gb*da) / 255.0
		db = (b1*a1 - b2*a2 - bb*da) / 255.0
	}

	y := dr*0.29889531 + dg*0.58662247 + db*0.11448223
	if yOnly {
		return y
	}

	i := dr*0.59597799 - dg*0.27417610 - db*0.32180189
	q := dr*0.21147017 - dg*0.52261711 + db*0.31114694

	delta := 0.5053*y*y + 0.299*i*i + 0.1957*q*q
	if y > 0 {
		return -delta
	}
	return delta
}

func isAntiAliased(img image.Image, x1, y1, width, height int, img2 image.Image) bool {
	x0 := max(x1-1, 0)
	y0 := max(y1-1, 0)
	x2 := min(x1+1, width-1)
	y2 := min(y1+1, height-1)

	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	min := 0.0
	max := 0.0
	minX, minY := 0, 0
	maxX, maxY := 0, 0

	r1, g1, b1, a1 := img.At(x1, y1).RGBA()

	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			r2, g2, b2, a2 := img.At(x, y).RGBA()
			pixel1 := rgba{R: r1, G: g1, B: b1, A: a1}
			pixel2 := rgba{R: r2, G: g2, B: b2, A: a2}
			delta := colorDelta(&pixel1, &pixel2, y*width+x, true)

			if delta == 0 {
				zeroes++
				if zeroes > 2 {
					return false
				}
			} else if delta < min {
				min = delta
				minX = x
				minY = y
			} else if delta > max {
				max = delta
				maxX = x
				maxY = y
			}
		}
	}

	if min == 0 || max == 0 {
		return false
	}

	return (hasManySiblings(img, minX, minY, width, height) && hasManySiblings(img2, minX, minY, width, height)) ||
		(hasManySiblings(img, maxX, maxY, width, height) && hasManySiblings(img2, maxX, maxY, width, height))
}

func hasManySiblings(img image.Image, x1, y1, width, height int) bool {
	x0 := max(x1-1, 0)
	y0 := max(y1-1, 0)
	x2 := min(x1+1, width-1)
	y2 := min(y1+1, height-1)
	valR, valG, valB, valA := img.At(x1, y1).RGBA()
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			r, g, b, a := img.At(x, y).RGBA()
			if valR == r && valG == g && valB == b && valA == a {
				zeroes++
				if zeroes > 2 {
					return true
				}
			}
		}
	}
	return false
}

func isIdentical(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}

	switch x := a.(type) {
	case *image.RGBA:
		y, ok := b.(*image.RGBA)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	case *image.RGBA64:
		y, ok := b.(*image.RGBA64)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	case *image.NRGBA:
		y, ok := b.(*image.NRGBA)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	case *image.NRGBA64:
		y, ok := b.(*image.NRGBA64)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	case *image.Gray:
		y, ok := b.(*image.Gray)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	case *image.Gray16:
		y, ok := b.(*image.Gray16)
		if ok && bytes.Equal(x.Pix, y.Pix) {
			return true
		}
	}
	return false
}
