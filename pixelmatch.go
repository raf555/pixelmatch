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

	var out *image.RGBA
	if options.writeTo != nil {
		out = image.NewRGBA(a.Bounds())
		defer func() {
			if err == nil {
				*options.writeTo = out
			}
		}()
	}
	aa := options.alpha

	if isIdentical(a, b) {
		if out != nil && !options.diffMask {
			rect := a.Bounds()
			for y := rect.Min.Y; y < rect.Max.Y; y++ {
				for x := rect.Min.X; x < rect.Max.X; x++ {
					r, g, b, aPix := a.At(x, y).RGBA()
					rf := float64(r) / 257.0
					gf := float64(g) / 257.0
					bf := float64(b) / 257.0
					af := float64(aPix) / 257.0

					yVal := rf*0.29889531 + gf*0.58662247 + bf*0.11448223
					val := uint8(255 + (yVal-255)*aa*af/255.0)
					out.SetRGBA(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
				}
			}
		}
		return 0, nil
	}

	maxDelta := 35215 * options.threshold * options.threshold

	rect := a.Bounds()
	y := rect.Min.Y
	ar := newImageLineReader(a, y)
	br := newImageLineReader(b, y)

	for ; ar.Next() && br.Next(); y++ {
		aLine := ar.Line()
		bLine := br.Line()

		for i := range aLine {
			x := rect.Min.X + i
			pos := y*rect.Dx() + x
			delta := colorDelta(&aLine[i], &bLine[i], pos, false)

			if math.Abs(delta) > maxDelta {
				if !options.includeAA && (isAntiAliased(ar, br, x, y) || isAntiAliased(br, ar, x, y)) {
					if out != nil && !options.diffMask {
						c := options.antiAliasedColor
						out.SetRGBA(x, y, c)
					}
				} else {
					if out != nil {
						if delta < 0 && options.diffColorAlt != nil {
							c := *options.diffColorAlt
							out.SetRGBA(x, y, c)
						} else {
							c := options.diffColor
							out.SetRGBA(x, y, c)
						}
					}
					diff++
				}
			} else if out != nil && !options.diffMask {
				r, g, b, aPix := aLine[i].R, aLine[i].G, aLine[i].B, aLine[i].A
				rf := float64(r) / 257.0
				gf := float64(g) / 257.0
				bf := float64(b) / 257.0
				af := float64(aPix) / 257.0

				yVal := rf*0.29889531 + gf*0.58662247 + bf*0.11448223
				val := uint8(255 + (yVal-255)*aa*af/255.0)
				out.SetRGBA(x, y, color.RGBA{R: val, G: val, B: val, A: 255})
			}
		}
	}

	return diff, nil
}

func checkerboardBackground(pos int) (float64, float64, float64) {
	rb := float64(48 + 159*(pos%2))
	gb := float64(48 + 159*((int(float64(pos)/1.618033988749895))%2))
	bb := float64(48 + 159*((int(float64(pos)/2.618033988749895))%2))
	return rb, gb, bb
}

func colorDelta(a, b *rgba, pos int, yOnly bool) float64 {
	dr := float64(a.R>>8) - float64(b.R>>8)
	dg := float64(a.G>>8) - float64(b.G>>8)
	db := float64(a.B>>8) - float64(b.B>>8)
	da := float64(a.A>>8) - float64(b.A>>8)

	if dr == 0 && dg == 0 && db == 0 && da == 0 {
		return 0
	}

	if a.A < 0xffff || b.A < 0xffff {
		rb, gb, bb := checkerboardBackground(pos)
		dr = (float64(a.R>>8)*float64(a.A>>8) - float64(b.R>>8)*float64(b.A>>8) - rb*da) / 255.0
		dg = (float64(a.G>>8)*float64(a.A>>8) - float64(b.G>>8)*float64(b.A>>8) - gb*da) / 255.0
		db = (float64(a.B>>8)*float64(a.A>>8) - float64(b.B>>8)*float64(b.A>>8) - bb*da) / 255.0
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

func isAntiAliased(a, b *imageLineReader, x1, y1 int) bool {
	r := a.Bounds()
	x0 := max(x1-1, r.Min.X)
	y0 := max(y1-1, r.Min.Y)
	x2 := min(x1+1, r.Max.X-1)
	y2 := min(y1+1, r.Max.Y-1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	min := 0.0
	max := 0.0
	var minX, minY, maxX, maxY int
	c := a.At(x1, y1)
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			pos := y*r.Dx() + x
			delta := colorDelta(c, a.At(x, y), pos, true)
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

	if max == 0 || min == 0 {
		return false
	}

	return (hasManySiblings(a, minX, minY) && hasManySiblings(b, minX, minY)) || (hasManySiblings(a, maxX, maxY) && hasManySiblings(b, maxX, maxY))
}

func hasManySiblings(img *imageLineReader, x1, y1 int) bool {
	rect := img.Bounds()
	x0 := max(x1-1, rect.Min.X)
	y0 := max(y1-1, rect.Min.Y)
	x2 := min(x1+1, rect.Max.X-1)
	y2 := min(y1+1, rect.Max.Y-1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	a := img.At(x1, y1)
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}

			b := img.At(x, y)
			if a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A {
				zeroes++
			}
			if zeroes > 2 {
				return true
			}
		}
	}
	return false
}

func isIdentical(a, b image.Image) bool {
	switch x := a.(type) {
	case *image.RGBA:
		y, ok := b.(*image.RGBA)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	case *image.RGBA64:
		y, ok := b.(*image.RGBA64)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	case *image.NRGBA:
		y, ok := b.(*image.NRGBA)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	case *image.NRGBA64:
		y, ok := b.(*image.NRGBA64)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	case *image.Gray:
		y, ok := b.(*image.Gray)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	case *image.Gray16:
		y, ok := b.(*image.Gray16)
		if ok && equals(x.Pix, y.Pix, x.Stride, y.Stride, x.Rect) {
			return true
		}
	}
	return false
}

func equals(pixA, pixB []uint8, strideA, strideB int, rect image.Rectangle) bool {
	w := rect.Dx()
	h := rect.Dy()
	if w*h*4 == len(pixA) && w*h*4 == len(pixB) {
		return bytes.Equal(pixA, pixB)
	}
	for y := range h {
		if !bytes.Equal(pixA[y*strideA:y*strideA+strideA], pixB[y*strideB:y*strideB+strideB]) {
			return false
		}
	}
	return true
}

func readLine(dst []rgba, img image.Image, y int) {
	rect := img.Bounds()
	switch v := img.(type) {
	case *image.RGBA:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			offset := lineOffset + i*4
			s := v.Pix[offset : offset+4 : offset+4]
			r := uint32(s[0])
			g := uint32(s[1])
			b := uint32(s[2])
			a := uint32(s[3])
			dst[i] = rgba{r<<8 | r, g<<8 | g, b<<8 | b, a<<8 | a}
		}
	case *image.RGBA64:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			offset := lineOffset + i*8
			s := v.Pix[offset : offset+8 : offset+8]
			r := uint32(s[0])<<8 | uint32(s[1])
			g := uint32(s[2])<<8 | uint32(s[3])
			b := uint32(s[4])<<8 | uint32(s[5])
			a := uint32(s[6])<<8 | uint32(s[7])
			dst[i] = rgba{r, g, b, a}
		}
	case *image.NRGBA:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			offset := lineOffset + i*4
			s := v.Pix[offset : offset+4 : offset+4]
			r := uint32(s[0])
			g := uint32(s[1])
			b := uint32(s[2])
			a := uint32(s[3])
			if a == 0xff {
				dst[i] = rgba{r<<8 | r, g<<8 | g, b<<8 | b, 0xffff}
			} else {
				dst[i] = rgba{(r<<8 | r) * a / 0xff, (g<<8 | g) * a / 0xff, (b<<8 | b) * a / 0xff, a<<8 | a}
			}
		}
	case *image.NRGBA64:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			offset := lineOffset + i*8
			s := v.Pix[offset : offset+8 : offset+8]
			r := uint32(s[0])<<8 | uint32(s[1])
			g := uint32(s[2])<<8 | uint32(s[3])
			b := uint32(s[4])<<8 | uint32(s[5])
			a := uint32(s[6])<<8 | uint32(s[7])
			dst[i] = rgba{r * a / 0xffff, g * a / 0xffff, b * a / 0xffff, a}
		}
	case *image.Gray:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			y := uint32(v.Pix[lineOffset+i])
			y |= y << 8
			dst[i] = rgba{y, y, y, 0xffff}
		}
	case *image.Gray16:
		lineOffset := v.PixOffset(rect.Min.X, y)
		for i := range dst {
			offset := lineOffset + i*2
			s := v.Pix[offset : offset+2 : offset+2]
			y := uint32(s[0])<<8 | uint32(s[1])
			dst[i] = rgba{y, y, y, 0xffff}
		}
	default:
		for i := range dst {
			r, g, b, a := v.At(rect.Min.X+i, y).RGBA()
			dst[i] = rgba{r, g, b, a}
		}
	}
}

type imageLineReader struct {
	image image.Image

	rect  image.Rectangle
	width int

	y     int
	lines [5][]rgba
}

func newImageLineReader(img image.Image, y int) *imageLineReader {
	rect := img.Bounds()
	width := rect.Dx()
	return &imageLineReader{
		image: img,
		rect:  rect,
		width: width,
		y:     y,
	}
}

func (r *imageLineReader) Next() bool {
	if r.y == r.rect.Max.Y {
		return false
	}
	if r.lines[2] == nil {
		for i := range r.lines {
			y := r.y + i - 2
			if r.rect.Min.Y <= y && y < r.rect.Max.Y {
				line := make([]rgba, r.width)
				readLine(line, r.image, y)
				r.lines[i] = line
			}
		}
	} else {
		old := r.lines[0]
		r.lines[0] = r.lines[1]
		r.lines[1] = r.lines[2]
		r.lines[2] = r.lines[3]
		r.lines[3] = r.lines[4]
		r.lines[4] = old
		if r.rect.Min.Y <= r.y+2 && r.y+2 < r.rect.Max.Y {
			if r.lines[4] == nil {
				r.lines[4] = make([]rgba, r.width)
			}
			readLine(r.lines[4], r.image, r.y+2)
		} else {
			r.lines[4] = nil
		}
	}
	r.y++
	return true
}

func (r *imageLineReader) Line() []rgba {
	return r.lines[2]
}

func (r *imageLineReader) Y() int {
	return r.y - 1
}

func (r *imageLineReader) At(x, y int) *rgba {
	return &r.lines[y-r.Y()+2][x]
}

func (r *imageLineReader) Bounds() image.Rectangle {
	return r.rect
}
