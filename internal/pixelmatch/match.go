// Package pixelmatch implements the comparison on raw RGBA byte slices. The
// public package wraps it with an image.Image API and functional options.
package pixelmatch

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Options configures a Match call. Use DefaultOptions() to get sensible
// defaults; the zero value of Options is NOT a useful default.
type Options struct {
	// Threshold is the matching threshold (0..1). Smaller values make the
	// comparison more sensitive. It is the maximum acceptable OKLab HyAB
	// distance between two colors, where 1.0 is the distance between black
	// and white. Default 0.1.
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

	// WindowSize, if positive, makes Match return the largest number of
	// differing pixels found in any WindowSize x WindowSize region instead
	// of the total count. The value is clamped to the image dimensions.
	// Zero (the default) or a negative value reports the total count.
	//
	// Deviation from upstream: mapbox/pixelmatch v8.0.0 defaults windowSize
	// to Infinity (total count) and treats any finite value, including 0
	// and negatives, as windowed mode clamped to a 1x1 window. Here 0 is the
	// field's zero value, so it means "unset". Use WindowSize = 1 for a 1x1
	// window.
	WindowSize int

	// IncludeAA, when true, disables anti-aliased pixel detection, so Match
	// counts AA pixels as differences. Default false.
	IncludeAA bool

	// HasDiffColorAlt enables the use of DiffColorAlt for darker pixels.
	HasDiffColorAlt bool

	// DiffMask, if true, draws the diff over a transparent background
	// instead of over the faded original image. Mask mode skips
	// anti-aliased pixels.
	DiffMask bool

	// Checkerboard, if true, blends semi-transparent pixels against a
	// checkerboard pattern when comparing, instead of plain white. The
	// checkerboard catches differences that only show up in alpha.
	Checkerboard bool

	// IgnoreMask, if non-nil, is a width*height byte slice (one byte per
	// pixel) that marks pixels to skip. Match treats a pixel with a non-zero
	// mask byte as matching: it leaves the pixel out of the count and draws
	// it like an unchanged pixel (faded grayscale, or untouched in DiffMask
	// mode). Use it for timestamps, animations, or other areas that change
	// on their own.
	IgnoreMask []byte
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

// Per-pixel mask states used by the windowed post-pass. Counted diffs get an
// odd value so the scan can test them with a single AND.
const (
	maskSame     uint8 = 0
	maskDiff     uint8 = 1
	maskExcluded uint8 = 2
)

// Match compares two RGBA images (4 bytes per pixel) of size width x height
// and returns the number of pixels that differ. If output is non-nil, it
// must be the same length as img1/img2, and a visual diff is written into
// it.
//
// If Options.WindowSize is positive, the return value is instead the largest
// number of differing pixels in any window of that size; see MaxWindowDiff
// in the package documentation.
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
	if o.IgnoreMask != nil && len(o.IgnoreMask) != width*height {
		return 0, ErrIgnoreMaskSizeMismatch
	}

	// Fast identical-images check with early exit. bytes.Equal compares the
	// whole buffer in SIMD chunks. Pixels covered by the ignore mask are
	// treated as matching regardless of their actual bytes, so that case
	// needs a per-pixel loop.
	pixels := width * height
	identical := o.IgnoreMask == nil && bytes.Equal(img1, img2)
	if o.IgnoreMask != nil {
		identical = true
		for i := range pixels {
			if o.IgnoreMask[i] != 0 {
				continue
			}
			if !equalPixel(img1, img2, i*4) {
				identical = false
				break
			}
		}
	}

	if identical {
		if output != nil && !o.DiffMask {
			drawGray(img1, output, o.Alpha)
		}
		return 0, nil
	}

	// Maximum acceptable OKLab HyAB distance between two colors. Black to
	// white measures 1.0, so the threshold needs no scaling.
	maxDelta := o.Threshold

	aaR, aaG, aaB := o.AAColor[0], o.AAColor[1], o.AAColor[2]
	diffR, diffG, diffB := o.DiffColor[0], o.DiffColor[1], o.DiffColor[2]
	altR, altG, altB := diffR, diffG, diffB
	if o.HasDiffColorAlt {
		altR, altG, altB = o.DiffColorAlt[0], o.DiffColorAlt[1], o.DiffColorAlt[2]
	}

	// The per-pixel mask is only allocated in windowed mode, keeping the
	// default path allocation-free.
	var mask []uint8
	if o.WindowSize > 0 {
		mask = make([]uint8, width*height)
	}
	// First and last row holding a counted diff, to bound the window scan.
	firstDiffY, lastDiffY := -1, 0

	// Fade the whole image up front, then paint diff and AA pixels over it.
	// One tight loop beats a per-pixel call, since drawGray is too large to
	// inline.
	if output != nil && !o.DiffMask {
		drawGray(img1, output, o.Alpha)
	}

	cache := oklabCachePool.Get().(*oklabCache)
	defer oklabCachePool.Put(cache)

	diff := 0
	for i := 0; i < pixels; i++ {
		pos := i * 4
		// Unchanged pixels need no work at all, so test two at a time.
		if o.IgnoreMask == nil && i+1 < pixels && binary.LittleEndian.Uint64(img1[pos:pos+8]) == binary.LittleEndian.Uint64(img2[pos:pos+8]) {
			i++
			continue
		}

		// Skip the color math when the 4-byte pixel words are identical, a
		// common case in screenshot diffs, or when the ignore mask covers
		// the pixel.
		var delta int
		if (o.IgnoreMask == nil || o.IgnoreMask[i] == 0) && !equalPixel(img1, img2, pos) {
			delta = colorDelta(img1, img2, pos, pos, o.Checkerboard, maxDelta, cache)
		}

		if delta != 0 {
			x := i % width
			y := i / width

			isExcludedAA := !o.IncludeAA &&
				(antialiased(img1, x, y, width, height, img2) ||
					antialiased(img2, x, y, width, height, img1))

			if isExcludedAA {
				// One of the pixels is anti-aliasing: draw it in the AA
				// color and do not count it as a difference.
				if output != nil && !o.DiffMask {
					drawPixel(output, pos, aaR, aaG, aaB)
				}
				if mask != nil {
					mask[i] = maskExcluded
				}
			} else {
				if output != nil {
					if delta < 0 {
						drawPixel(output, pos, altR, altG, altB)
					} else {
						drawPixel(output, pos, diffR, diffG, diffB)
					}
				}
				if mask != nil {
					mask[i] = maskDiff
					if firstDiffY < 0 {
						firstDiffY = y
					}
					lastDiffY = y
				}
				diff++
			}
		}
	}

	if mask == nil {
		return diff, nil
	}
	return maxWindowDiff(mask, width, height, o.WindowSize, firstDiffY, lastDiffY), nil
}

// maxWindowDiff returns the largest number of diff pixels (mask value
// maskDiff) contained in any n x n window, with n clamped to the image
// dimensions. Anti-aliased pixels are excluded by construction, since only
// counted diffs are given an odd mask value.
//
// The scan keeps a per-column count over a sliding band of n rows, updated
// incrementally as rows enter and leave, then slides a horizontal running
// sum across it. Bands whose total cannot beat the current maximum skip the
// horizontal pass, which is most of them for a typical sparse diff.
func maxWindowDiff(mask []uint8, width, height, windowSize, firstDiffY, lastDiffY int) int {
	if firstDiffY < 0 {
		return 0 // the AA detector excluded each difference
	}

	n := min(windowSize, width, height)
	n = max(n, 1)

	colSum := make([]int32, width)
	maxCount, bandTotal := 0, 0

	// Rows before firstDiffY are empty, so starting the scan there keeps
	// colSum correct without any special initialization; rows after
	// lastDiffY only matter until the band has drained.
	yEnd := min(height-1, lastDiffY+n-1)

	for y := firstDiffY; y <= yEnd; y++ {
		rowStart := y * width
		leaving := y - n

		if leaving >= 0 {
			leavingStart := leaving * width
			for x := range width {
				d := int32(mask[rowStart+x]&1) - int32(mask[leavingStart+x]&1)
				colSum[x] += d
				bandTotal += int(d)
			}
		} else {
			for x := range width {
				e := int32(mask[rowStart+x] & 1)
				colSum[x] += e
				bandTotal += int(e)
			}
			// Only scan windows that fit vertically.
			if y < n-1 {
				continue
			}
		}

		// No window in this band can hold more than the band itself.
		if bandTotal <= maxCount {
			continue
		}

		windowSum := 0
		for x := range n - 1 {
			windowSum += int(colSum[x])
		}
		for x := n - 1; x < width; x++ {
			windowSum += int(colSum[x])
			if windowSum > maxCount {
				maxCount = windowSum
			}
			windowSum -= int(colSum[x-n+1])
		}
	}

	return maxCount
}

// equalPixel reports whether the 4-byte RGBA pixel at the same offset in
// img1 and img2 is identical. It runs for every pixel, and the two Uint32
// loads compile to a single inlined compare. bytes.Equal calls
// runtime.memequal instead and benchmarked 16-40% slower; comparing as
// [4]byte was 2-3% slower on images that differ.
func equalPixel(img1, img2 []byte, pos int) bool {
	return binary.LittleEndian.Uint32(img1[pos:pos+4]) ==
		binary.LittleEndian.Uint32(img2[pos:pos+4])
}

// antialiased reports whether the pixel at (x1, y1) in img is likely part of
// anti-aliasing, by inspecting its 8 neighbors and corresponding pixels in
// img2. Based on Vyšniauskas (2009).
func antialiased(img []byte, x1, y1, width, height int, img2 []byte) bool {
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
			delta := brightnessDelta(img, (y*width+x)*4, cr, cg, cb, ca)

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
	pos1 := (y1*width + x1) * 4
	center := binary.LittleEndian.Uint32(img[pos1 : pos1+4])

	// Interior pixels have all 8 neighbors, so the walk can be unrolled
	// with no per-neighbor edge handling.
	if x1 > 0 && x1 < width-1 && y1 > 0 && y1 < height-1 {
		row := width * 4
		eq := b2i(center == binary.LittleEndian.Uint32(img[pos1-row-4:pos1-row])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1-4:pos1])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1+row-4:pos1+row])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1-row:pos1-row+4])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1+row:pos1+row+4])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1-row+4:pos1-row+8])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1+4:pos1+8])) +
			b2i(center == binary.LittleEndian.Uint32(img[pos1+row+4:pos1+row+8]))
		return eq > 2
	}

	x0 := max(x1-1, 0)
	y0 := max(y1-1, 0)
	x2 := min(x1+1, width-1)
	y2 := min(y1+1, height-1)

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

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// brightnessDelta is the brightness-only color delta used by the AA
// detector, with the center pixel's RGBA hoisted out of the neighbor loop.
//
// It stays on gamma-space Rec.601 luma rather than the OKLab distance used
// by colorDelta: the detector only needs a cheap monotonic scalar to find
// the direction of an intensity ramp (its darkest and brightest neighbor),
// and OKLab lightness both detects AA worse in dark regions and costs far
// more, being evaluated up to 16 times per candidate pixel.
//
// Semi-transparent pixels are composited over fixed white rather than the
// checkerboard used by colorDelta: a ramp structure test needs a
// deterministic background, and a pseudo-random per-position one distorts
// the very ramps it looks for. When the composited luma delta cancels out
// exactly but alpha differs, the alpha delta gives the ramp direction, so
// only pixels equal in both premultiplied luma and alpha count as equal
// siblings.
func brightnessDelta(img []byte, m int, r1b, g1b, b1b, a1b uint8) float64 {
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
		// Products are wrapped in float64() to block FMA fusion; see the
		// note at the top of oklab.go.
		dr = (float64(r1*a1) - float64(r2*a2) - float64(255*da)) / 255
		dg = (float64(g1*a1) - float64(g2*a2) - float64(255*da)) / 255
		db = (float64(b1*a1) - float64(b2*a2) - float64(255*da)) / 255
		d := float64(dr*0.29889531) + float64(dg*0.58662247) + float64(db*0.11448223)
		if d == 0 && da != 0 {
			return da / 2
		}
		return d
	}

	return float64(dr*0.29889531) + float64(dg*0.58662247) + float64(db*0.11448223)
}

// drawPixel writes an opaque RGB pixel at byte offset pos in output.
func drawPixel(output []byte, pos int, r, g, b uint8) {
	output[pos] = r
	output[pos+1] = g
	output[pos+2] = b
	output[pos+3] = 255
}

// drawGray writes a faded grayscale version of every pixel in img into
// output.
func drawGray(img, output []byte, alpha float64) {
	for i := 0; i+4 <= len(img) && i+4 <= len(output); i += 4 {
		r := float64(img[i])
		g := float64(img[i+1])
		b := float64(img[i+2])
		a := float64(img[i+3])
		// Products are wrapped in float64() to block FMA fusion; see the
		// note at the top of oklab.go.
		val := 255 + float64((float64(r*0.29889531)+float64(g*0.58662247)+float64(b*0.11448223)-255)*alpha*a/255)
		v := clampUint8(val)
		output[i] = v
		output[i+1] = v
		output[i+2] = v
		output[i+3] = 255
	}
}

// clampUint8 converts x to a byte, truncating toward zero like upstream's
// Uint8Array store. Values outside 0..255 clamp to the nearest end, and NaN
// becomes 0. Those only occur when Alpha is outside 0..1; upstream wraps
// them modulo 256 instead.
func clampUint8(x float64) uint8 {
	// min and max return NaN for a NaN input, so handle it first.
	if math.IsNaN(x) {
		return 0
	}
	return uint8(min(max(x, 0), 255))
}
