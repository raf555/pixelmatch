package pixelmatch

import "math"

// This file implements the perceptual color difference used by Match: the
// OKLab color space (Ottosson, 2020) with the HyAB metric (Abasi et al.,
// 2019), |ΔLr| + √(Δa² + Δb²), which tracks large color differences better
// than the YIQ metric used before.
//
// Lightness is Ottosson's toe-corrected Lr rather than raw OKLab L. Lr
// avoids over-expanding near-black differences and keeps the black-to-white
// distance at 1.0, so the threshold option serves as the maximum acceptable
// distance with no scaling.
//
// The reference implementation approximates sRGB-to-linear conversion and
// the cube root with linearly interpolated tables. This file reproduces
// those tables to keep its output byte-for-byte identical to the JavaScript,
// so they serve correctness as well as speed.

// The tables use Go's math.Pow and math.Cbrt, which can differ from
// JavaScript's by up to 2 ULP on some entries. JavaScript itself has no
// single answer here: since Node 24, V8 sends Math.pow to the C library's
// pow, so upstream's own tables vary by operating system. The test suite
// still matches upstream's output byte for byte.

// Fused multiply-add: the Go spec lets the compiler fuse x*y + z into a
// single FMA instruction (it does on arm64, ppc64, s390x, riscv64, and amd64
// with GOAMD64=v3), which skips the intermediate rounding JavaScript
// performs. Each product that feeds an addition or subtraction in this file,
// brightnessDelta and drawGrayPixel sits inside an explicit float64(...)
// conversion. The spec guarantees that conversion rounds, which blocks
// fusion. Keep the pattern when you change this arithmetic, or results will
// differ by architecture.

// linTable maps an sRGB byte to linear light in [0..1]. It carries a 257th
// entry (a copy of the last) so fractional channel values produced by alpha
// blending can be interpolated without a bounds check.
var linTable [257]float64

// Per-channel LMS matrix contributions, premultiplied for each opaque sRGB
// byte value. Summing three of these replaces a matrix multiply on the hot
// opaque path.
var (
	lmsLR, lmsLG, lmsLB [256]float64
	lmsMR, lmsMG, lmsMB [256]float64
	lmsSR, lmsSG, lmsSB [256]float64
)

// cbrtN is the resolution of the cube-root table over [0..1]; cbrtTable has
// two extra entries so interpolation at x == 1 stays in bounds.
const cbrtN = 4096

// cbrtExact is the first table slot where linear interpolation stays accurate
// enough. Below it the cbrt curve bends too hard, and interpolation
// over-weights differences between near-black pixels by up to 50%. For x in
// the first cbrtExact slots, cbrtLUT falls back to math.Cbrt.
const cbrtExact = 8

var cbrtTable [cbrtN + 2]float64

// Toe correction constants (Ottosson).
const (
	toeK1 = 0.206
	toeK2 = 0.03
	toeK3 = (1 + toeK1) / (1 + toeK2)
)

func init() {
	for i := range 256 {
		c := float64(i) / 255
		if c <= 0.04045 {
			linTable[i] = c / 12.92
		} else {
			linTable[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	linTable[256] = linTable[255]

	for i := range 256 {
		lr := linTable[i]
		lmsLR[i] = 0.4122214708 * lr
		lmsMR[i] = 0.2119034982 * lr
		lmsSR[i] = 0.0883024619 * lr
		lmsLG[i] = 0.5363325363 * lr
		lmsMG[i] = 0.6806995451 * lr
		lmsSG[i] = 0.2817188376 * lr
		lmsLB[i] = 0.0514459929 * lr
		lmsMB[i] = 0.1073969566 * lr
		lmsSB[i] = 0.6299787005 * lr
	}

	for i := range cbrtN + 2 {
		cbrtTable[i] = math.Cbrt(float64(i) / cbrtN)
	}
}

// linLUT converts a fractional [0..255] channel value to linear light by
// interpolating linTable.
func linLUT(x float64) float64 {
	i := int(x)
	return linTable[i] + float64((linTable[i+1]-linTable[i])*(x-float64(i)))
}

// cbrtLUT approximates the cube root over [0..1] by interpolating cbrtTable.
// For x in the first cbrtExact slots, interpolation is too coarse to represent
// the near-zero curvature of cbrt, so it computes the exact cube root instead.
func cbrtLUT(x float64) float64 {
	// Fusion may cross statements, so t itself must be rounded before the
	// t-float64(i) below.
	t := float64(x * cbrtN)
	i := int(t)
	if i < cbrtExact {
		return math.Cbrt(x)
	}
	return cbrtTable[i] + float64((cbrtTable[i+1]-cbrtTable[i])*(t-float64(i)))
}

// lightness returns Ottosson's toe-corrected OKLab lightness Lr from the
// cube-rooted LMS values.
func lightness(l, m, s float64) float64 {
	ok := float64(0.2104542553*l) + float64(0.7936177850*m) - float64(0.0040720468*s)
	x := float64(toeK3*ok) - toeK1
	// Round the result too: once inlined, the final multiply would otherwise
	// fuse with the caller's lr1-lr2 subtraction.
	return float64(0.5 * (x + math.Sqrt(float64(x*x)+float64(4*toeK2*toeK3*ok))))
}

// colorDelta reports whether the OKLab HyAB distance between the pixels at
// img1[k:] and img2[m:] exceeds maxDelta: 0 if it does not, otherwise ±1,
// negative when the img2 pixel is the darker of the two.
//
// The caller must guarantee the two pixels differ, because colorDelta skips
// the early-zero check.
func colorDelta(img1, img2 []byte, k, m int, checkerboard bool, maxDelta float64) int {
	r1, g1, b1, a1 := img1[k], img1[k+1], img1[k+2], img1[k+3]
	r2, g2, b2, a2 := img2[m], img2[m+1], img2[m+2], img2[m+3]

	if a1 == 255 && a2 == 255 {
		return colorDeltaOpaque(r1, g1, b1, r2, g2, b2, maxDelta)
	}
	return colorDeltaTransparent(r1, g1, b1, a1, r2, g2, b2, a2, k, checkerboard, maxDelta)
}

// colorDeltaOpaque is the fast path for opaque pixels, where the per-byte
// LMS tables replace the sRGB-to-linear conversion.
//
// Deviation from upstream: mapbox/pixelmatch v8.0.0 also memoizes each
// opaque color's LMS and Lr values in a module-level direct-mapped cache.
// This port leaves it out on purpose: a package-level cache is shared
// mutable state and would race when Match runs on several goroutines. The
// cache only memoizes, so dropping it changes speed, not results.
func colorDeltaOpaque(r1, g1, b1, r2, g2, b2 uint8, maxDelta float64) int {
	l1 := cbrtLUT(lmsLR[r1] + lmsLG[g1] + lmsLB[b1])
	m1 := cbrtLUT(lmsMR[r1] + lmsMG[g1] + lmsMB[b1])
	s1 := cbrtLUT(lmsSR[r1] + lmsSG[g1] + lmsSB[b1])
	lr1 := lightness(l1, m1, s1)

	l2 := cbrtLUT(lmsLR[r2] + lmsLG[g2] + lmsLB[b2])
	m2 := cbrtLUT(lmsMR[r2] + lmsMG[g2] + lmsMB[b2])
	s2 := cbrtLUT(lmsSR[r2] + lmsSG[g2] + lmsSB[b2])
	lr2 := lightness(l2, m2, s2)

	return oklabHyabDelta(lr1-lr2, l1-l2, m1-m2, s1-s2, maxDelta)
}

// colorDeltaTransparent composites both pixels over a background before
// comparing them, since a semi-transparent color has no meaning on its own.
// The background is the checkerboard pattern by default, or plain white.
func colorDeltaTransparent(r1, g1, b1, a1, r2, g2, b2, a2 uint8, k int, checkerboard bool, maxDelta float64) int {
	rb, gb, bb := 255.0, 255.0, 255.0
	if checkerboard {
		rb, gb, bb = checkerboardBackground(k)
	}

	af1, af2 := float64(a1), float64(a2)
	// Blended channel values are fractional, hence the interpolated LUT.
	cr1 := (float64(float64(r1)*af1) + float64(rb*(255-af1))) / 255
	cg1 := (float64(float64(g1)*af1) + float64(gb*(255-af1))) / 255
	cb1 := (float64(float64(b1)*af1) + float64(bb*(255-af1))) / 255
	cr2 := (float64(float64(r2)*af2) + float64(rb*(255-af2))) / 255
	cg2 := (float64(float64(g2)*af2) + float64(gb*(255-af2))) / 255
	cb2 := (float64(float64(b2)*af2) + float64(bb*(255-af2))) / 255

	lr1, lg1, lb1 := linLUT(cr1), linLUT(cg1), linLUT(cb1)
	lr2, lg2, lb2 := linLUT(cr2), linLUT(cg2), linLUT(cb2)

	l1 := cbrtLUT(float64(0.4122214708*lr1) + float64(0.5363325363*lg1) + float64(0.0514459929*lb1))
	m1 := cbrtLUT(float64(0.2119034982*lr1) + float64(0.6806995451*lg1) + float64(0.1073969566*lb1))
	s1 := cbrtLUT(float64(0.0883024619*lr1) + float64(0.2817188376*lg1) + float64(0.6299787005*lb1))
	l2 := cbrtLUT(float64(0.4122214708*lr2) + float64(0.5363325363*lg2) + float64(0.0514459929*lb2))
	m2 := cbrtLUT(float64(0.2119034982*lr2) + float64(0.6806995451*lg2) + float64(0.1073969566*lb2))
	s2 := cbrtLUT(float64(0.0883024619*lr2) + float64(0.2817188376*lg2) + float64(0.6299787005*lb2))

	lrA := lightness(l1, m1, s1)
	lrB := lightness(l2, m2, s2)

	return oklabHyabDelta(lrA-lrB, l1-l2, m1-m2, s1-s2, maxDelta)
}

// oklabHyabDelta compares the HyAB distance against maxDelta without taking
// a square root: the distance stays below the threshold exactly when
// |ΔLr| <= maxDelta and Δa² + Δb² <= (maxDelta − |ΔLr|)².
//
// Only the threshold test and the lighter/darker sign are ever used, so the
// result is reported as 0 or ±1 rather than an actual distance.
func oklabHyabDelta(dLr, dl, dm, ds, maxDelta float64) int {
	if rest := maxDelta - math.Abs(dLr); rest >= 0 {
		da := float64(1.9779984951*dl) - float64(2.4285922050*dm) + float64(0.4505937099*ds)
		db := float64(0.0259040371*dl) + float64(0.7827717662*dm) - float64(0.8086757660*ds)
		if float64(da*da)+float64(db*db) <= float64(rest*rest) {
			return 0
		}
	}
	if dLr > 0 {
		return -1
	}
	return 1
}

// checkerboardBackground returns the RGB background color for a
// semi-transparent pixel at byte offset k. The three channels pick one of
// eight {48, 207}³ colors from the top three bits of a Fibonacci hash of
// the pixel index, the same construction upstream mapbox/pixelmatch v8.0.0
// uses. No pair of colors that only differ in alpha can look alike over
// more than 1/8 of the pixels.
func checkerboardBackground(k int) (rb, gb, bb float64) {
	// JS: `Math.imul(k >> 2, 0x9e3779b1)` is a 32-bit signed multiply with
	// wrap-around; Go's uint32 multiplication produces the same bit pattern.
	h := uint32(k>>2) * 0x9e3779b1
	rb = 48 + float64(159*float64(h>>31))
	gb = 48 + float64(159*float64((h>>30)&1))
	bb = 48 + float64(159*float64((h>>29)&1))
	return
}
