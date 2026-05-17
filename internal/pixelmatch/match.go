package pixelmatch

import (
	"encoding/binary"
	"math"
)

// Options configures a Match call. Use DefaultOptions() to get sensible
// defaults; the zero value of Options is NOT a useful default.
type Options struct {
	// Threshold is the matching threshold (0..1). Smaller values make the
	// comparison more sensitive. Default 0.1.
	Threshold float64

	// Alpha is the blending factor of unchanged pixels in the diff output:
	// 0 = pure white, 1 = original brightness. Default 0.1.
	Alpha float64

	// AAColor is the RGB color used for anti-aliased pixels in the diff
	// output. Default [255, 255, 0] (yellow).
	AAColor [3]uint8

	// DiffColor is the RGB color used for differing pixels in the diff
	// output. Default [255, 0, 0] (red).
	DiffColor [3]uint8

	// DiffColorAlt is used for pixels in img2 that are darker than img1
	// (only when HasDiffColorAlt is true), letting you distinguish "added"
	// from "removed" content.
	DiffColorAlt [3]uint8

	// IncludeAA, when true, disables anti-aliased pixel detection (so AA
	// pixels are counted as real differences). Default false.
	IncludeAA bool

	// HasDiffColorAlt enables the use of DiffColorAlt for darker pixels.
	HasDiffColorAlt bool

	// DiffMask, if true, draws the diff over a transparent background
	// instead of over the (faded) original image. Anti-aliased pixels are
	// not drawn in mask mode.
	DiffMask bool

	// Checkerboard, if true, blends semi-transparent pixels against a
	// checkerboard pattern when comparing, instead of plain white. This
	// gives a more accurate alpha-aware comparison.
	Checkerboard bool
}

// DefaultOptions returns options matching the JS pixelmatch defaults.
func DefaultOptions() Options {
	return Options{
		Threshold:    0.1,
		IncludeAA:    false,
		Alpha:        0.1,
		AAColor:      [3]uint8{255, 255, 0},
		DiffColor:    [3]uint8{255, 0, 0},
		Checkerboard: true,
	}
}

// Match compares two RGBA images (4 bytes per pixel) of size width x height
// and returns the number of pixels that differ. If output is non-nil, it
// must be the same length as img1/img2, and a visual diff is written into
// it.
//
// img1, img2, and output (if provided) must all have length width*height*4.
func Match(img1, img2, output []byte, width, height int, opts *Options) (int, error) {
	o := DefaultOptions()
	if opts != nil {
		o = *opts
	}

	if width <= 0 || height <= 0 {
		return 0, ErrInvalidDimensions
	}
	expected := width * height * 4
	if len(img1) != expected || len(img2) != expected {
		return 0, ErrDataSizeMismatch
	}
	if output != nil && len(output) != expected {
		return 0, ErrOutputSizeMismatch
	}

	// Fast identical-images check with early exit.
	identical := true
	for i := range expected {
		if img1[i] != img2[i] {
			identical = false
			break
		}
	}

	if identical {
		if output != nil && !o.DiffMask {
			for pos := 0; pos < expected; pos += 4 {
				drawGrayPixel(img1, pos, o.Alpha, output)
			}
		}
		return 0, nil
	}

	// 35215 is the maximum possible value for the YIQ difference metric.
	maxDelta := 35215.0 * o.Threshold * o.Threshold

	aaR, aaG, aaB := o.AAColor[0], o.AAColor[1], o.AAColor[2]
	diffR, diffG, diffB := o.DiffColor[0], o.DiffColor[1], o.DiffColor[2]
	altR, altG, altB := diffR, diffG, diffB
	if o.HasDiffColorAlt {
		altR, altG, altB = o.DiffColorAlt[0], o.DiffColorAlt[1], o.DiffColorAlt[2]
	}

	diff := 0
	pos := 0
	pixels := width * height
	for i := range pixels {
		// Skip the YIQ math entirely when the 4-byte pixel words are
		// identical — a common case in screenshot diffs.
		var delta float64
		if !equalPixel(img1, img2, pos) {
			delta = colorDelta(img1, img2, pos, pos, o.Checkerboard)
		}

		if math.Abs(delta) > maxDelta {
			x := i % width
			y := i / width

			isExcludedAA := !o.IncludeAA &&
				(antialiased(img1, x, y, width, height, img2, o.Checkerboard) ||
					antialiased(img2, x, y, width, height, img1, o.Checkerboard))

			if isExcludedAA {
				if output != nil && !o.DiffMask {
					drawPixel(output, pos, aaR, aaG, aaB)
				}
			} else {
				if output != nil {
					if delta < 0 {
						drawPixel(output, pos, altR, altG, altB)
					} else {
						drawPixel(output, pos, diffR, diffG, diffB)
					}
				}
				diff++
			}
		} else if output != nil && !o.DiffMask {
			drawGrayPixel(img1, pos, o.Alpha, output)
		}

		pos += 4
	}

	return diff, nil
}

// equalPixel reports whether the 4-byte RGBA pixel at the same offset in
// img1 and img2 is identical, in one machine word compare.
func equalPixel(img1, img2 []byte, pos int) bool {
	return binary.LittleEndian.Uint32(img1[pos:pos+4]) ==
		binary.LittleEndian.Uint32(img2[pos:pos+4])
}

// antialiased reports whether the pixel at (x1, y1) in img is likely part of
// anti-aliasing, by inspecting its 8 neighbors and corresponding pixels in
// img2. Based on Vyšniauskas (2009).
func antialiased(img []byte, x1, y1, width, height int, img2 []byte, checkerboard bool) bool {
	x0 := max(x1-1, 0)
	y0 := max(y1-1, 0)
	x2 := min(x1+1, width-1)
	y2 := min(y1+1, height-1)
	pos4 := (y1*width + x1) * 4

	// Cache the center pixel's RGBA once; the inner loop is hot.
	cr := img[pos4]
	cg := img[pos4+1]
	cb := img[pos4+2]
	ca := img[pos4+3]

	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	var minDelta, maxDelta float64
	var minX, minY, maxX, maxY int

	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			delta := brightnessDelta(img, pos4, (y*width+x)*4, cr, cg, cb, ca, checkerboard)

			switch {
			case delta == 0:
				zeroes++
				if zeroes > 2 {
					return false
				}
			case delta < minDelta:
				minDelta = delta
				minX, minY = x, y
			case delta > maxDelta:
				maxDelta = delta
				maxX, maxY = x, y
			}
		}
	}

	// Need both a darker and a brighter neighbor for AA.
	if minDelta == 0 || maxDelta == 0 {
		return false
	}

	return (hasManySiblings(img, minX, minY, width, height) && hasManySiblings(img2, minX, minY, width, height)) ||
		(hasManySiblings(img, maxX, maxY, width, height) && hasManySiblings(img2, maxX, maxY, width, height))
}

// hasManySiblings reports whether the pixel at (x1, y1) has 3+ adjacent
// pixels with the exact same RGBA value (compared as 32-bit words).
func hasManySiblings(img []byte, x1, y1, width, height int) bool {
	x0 := max(x1-1, 0)
	y0 := max(y1-1, 0)
	x2 := min(x1+1, width-1)
	y2 := min(y1+1, height-1)
	centerPos := (y1*width + x1) * 4
	center := binary.LittleEndian.Uint32(img[centerPos : centerPos+4])

	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}

	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			p := (y*width + x) * 4
			if binary.LittleEndian.Uint32(img[p:p+4]) == center {
				zeroes++
			}
			if zeroes > 2 {
				return true
			}
		}
	}
	return false
}

// colorDelta returns the signed squared YIQ perceptual color distance
// between img1[k..k+4] and img2[m..m+4]. The sign encodes whether the img2
// pixel is darker (positive) or lighter (negative) than img1.
//
// Caller must guarantee the two pixels are not identical — the early-zero
// check is omitted here.
func colorDelta(img1, img2 []byte, k, m int, checkerboard bool) float64 {
	r1 := float64(img1[k])
	g1 := float64(img1[k+1])
	b1 := float64(img1[k+2])
	a1 := float64(img1[k+3])

	r2 := float64(img2[m])
	g2 := float64(img2[m+1])
	b2 := float64(img2[m+2])
	a2 := float64(img2[m+3])

	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	da := a1 - a2

	if a1 < 255 || a2 < 255 {
		rb, gb, bb := 255.0, 255.0, 255.0
		if checkerboard {
			rb, gb, bb = checkerboardBackground(k)
		}
		dr = (r1*a1 - r2*a2 - rb*da) / 255
		dg = (g1*a1 - g2*a2 - gb*da) / 255
		db = (b1*a1 - b2*a2 - bb*da) / 255
	}

	y := dr*0.29889531 + dg*0.58662247 + db*0.11448223
	i := dr*0.59597799 - dg*0.27417610 - db*0.32180189
	q := dr*0.21147017 - dg*0.52261711 + db*0.31114694

	delta := 0.5053*y*y + 0.299*i*i + 0.1957*q*q

	if y > 0 {
		return -delta
	}
	return delta
}

// brightnessDelta is the brightness-only variant used by the AA detector,
// with the center pixel's RGBA hoisted out of the neighbor loop.
func brightnessDelta(img []byte, k, m int, r1b, g1b, b1b, a1b uint8, checkerboard bool) float64 {
	r2 := float64(img[m])
	g2 := float64(img[m+1])
	b2 := float64(img[m+2])
	a2 := float64(img[m+3])

	r1 := float64(r1b)
	g1 := float64(g1b)
	b1 := float64(b1b)
	a1 := float64(a1b)

	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	da := a1 - a2

	if dr == 0 && dg == 0 && db == 0 && da == 0 {
		return 0
	}

	if a1 < 255 || a2 < 255 {
		rb, gb, bb := 255.0, 255.0, 255.0
		if checkerboard {
			rb, gb, bb = checkerboardBackground(k)
		}
		dr = (r1*a1 - r2*a2 - rb*da) / 255
		dg = (g1*a1 - g2*a2 - gb*da) / 255
		db = (b1*a1 - b2*a2 - bb*da) / 255
	}

	return dr*0.29889531 + dg*0.58662247 + db*0.11448223
}

// checkerboardBackground returns the RGB background color for a
// semi-transparent pixel at byte offset k.
// Each channel is either 48 or 207, producing a tri-tone
// noisy background that breaks alpha symmetries.
func checkerboardBackground(k int) (rb, gb, bb float64) {
	rb = 48 + 159*float64(k%2)
	// `(k / 1.618...) | 0` in JS truncates toward zero (works because k≥0).
	gb = 48 + 159*float64(int(float64(k)/1.618033988749895)%2)
	bb = 48 + 159*float64(int(float64(k)/2.618033988749895)%2)
	return
}

// drawPixel writes an opaque RGB pixel at byte offset pos in output.
func drawPixel(output []byte, pos int, r, g, b uint8) {
	output[pos] = r
	output[pos+1] = g
	output[pos+2] = b
	output[pos+3] = 255
}

// drawGrayPixel writes a faded grayscale version of img's pixel at offset i
// into output. The float-to-byte conversion uses JS ToUint8 semantics
// (truncate toward zero, then modulo 256) to match JavaScript Uint8Array
// assignment exactly.
func drawGrayPixel(img []byte, i int, alpha float64, output []byte) {
	r := float64(img[i])
	g := float64(img[i+1])
	b := float64(img[i+2])
	a := float64(img[i+3])
	val := 255 + (r*0.29889531+g*0.58662247+b*0.11448223-255)*alpha*a/255
	v := toUint8(val)
	output[i] = v
	output[i+1] = v
	output[i+2] = v
	output[i+3] = 255
}

// toUint8 converts a JS-style float-to-byte assignment: truncate toward
// zero, then take modulo 256. This matches `someUint8Array[k] = x` in
// JavaScript for any finite x. NaN becomes 0.
func toUint8(x float64) uint8 {
	if math.IsNaN(x) {
		return 0
	}
	t := math.Trunc(x)
	m := math.Mod(t, 256)
	if m < 0 {
		m += 256
	}
	return uint8(m)
}
