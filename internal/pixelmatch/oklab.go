package pixelmatch

import "math"

// This file implements the perceptual color difference used by Match: the
// OKLab color space (Ottosson, 2020) with the HyAB metric (Abasi et al.,
// 2019), |ΔLr| + √(Δa² + Δb²), which tracks large color differences much
// better than the YIQ metric used before.
//
// Lightness is Ottosson's toe-corrected Lr rather than raw OKLab L, which
// avoids over-expanding near-black differences while keeping the
// black-to-white distance at exactly 1.0. That is why the threshold option
// is now used directly as the maximum acceptable distance.
//
// The lookup tables below are not merely an optimization: the reference
// implementation approximates sRGB-to-linear conversion and the cube root
// with linearly interpolated tables, so reproducing them exactly is what
// keeps this port byte-for-byte identical to the JavaScript output.

// Caveat: Go's math.Pow and math.Cbrt disagree with V8's by 1 ULP on some
// table entries, so the tables here are within one ULP of the reference
// rather than bit-identical. A 14.7M-pixel cross-check across seven
// thresholds and both blending modes produced zero differing output bytes,
// but if exact parity ever needs to be structural rather than empirical,
// embed the reference table values verbatim instead of computing them.

// linTable maps an sRGB byte to linear light in [0..1]. It carries a 257th
// entry (a copy of the last) so fractional channel values produced by alpha
// blending can be interpolated without a bounds check.
var linTable [257]float64

// Per-channel LMS matrix contributions, premultiplied for every opaque sRGB
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

var cbrtTable [cbrtN + 2]float64

// Toe correction constants (Ottosson). These are float64 variables rather
// than untyped constants on purpose: Go would fold constant expressions in
// arbitrary precision, which rounds differently from the reference
// implementation's float64 arithmetic.
var (
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
	return linTable[i] + (linTable[i+1]-linTable[i])*(x-float64(i))
}

// cbrtLUT approximates the cube root over [0..1] by interpolating cbrtTable.
func cbrtLUT(x float64) float64 {
	t := x * cbrtN
	i := int(t)
	return cbrtTable[i] + (cbrtTable[i+1]-cbrtTable[i])*(t-float64(i))
}

// toe applies Ottosson's toe function, mapping OKLab L to the perceptually
// corrected lightness Lr.
func toe(l float64) float64 {
	x := toeK3*l - toeK1
	return 0.5 * (x + math.Sqrt(x*x+4*toeK2*toeK3*l))
}

// colorDelta reports whether the OKLab HyAB distance between the pixels at
// img1[k:] and img2[m:] exceeds maxDelta: 0 if it does not, otherwise ±1,
// negative when the img2 pixel is the lighter of the two.
//
// The caller must guarantee the two pixels are not identical — the
// early-zero check is omitted here.
func colorDelta(img1, img2 []byte, k, m int, checkerboard bool, maxDelta float64) int {
	r1, g1, b1, a1 := img1[k], img1[k+1], img1[k+2], img1[k+3]
	r2, g2, b2, a2 := img2[m], img2[m+1], img2[m+2], img2[m+3]

	if a1 == 255 && a2 == 255 {
		return colorDeltaOpaque(r1, g1, b1, r2, g2, b2, maxDelta)
	}
	return colorDeltaTransparent(r1, g1, b1, a1, r2, g2, b2, a2, k, checkerboard, maxDelta)
}

// colorDeltaOpaque is the fast path for fully opaque pixels, where the
// per-byte LMS tables remove the sRGB-to-linear conversion entirely.
func colorDeltaOpaque(r1, g1, b1, r2, g2, b2 uint8, maxDelta float64) int {
	l1 := cbrtLUT(lmsLR[r1] + lmsLG[g1] + lmsLB[b1])
	m1 := cbrtLUT(lmsMR[r1] + lmsMG[g1] + lmsMB[b1])
	s1 := cbrtLUT(lmsSR[r1] + lmsSG[g1] + lmsSB[b1])
	lr1 := toe(0.2104542553*l1 + 0.7936177850*m1 - 0.0040720468*s1)

	l2 := cbrtLUT(lmsLR[r2] + lmsLG[g2] + lmsLB[b2])
	m2 := cbrtLUT(lmsMR[r2] + lmsMG[g2] + lmsMB[b2])
	s2 := cbrtLUT(lmsSR[r2] + lmsSG[g2] + lmsSB[b2])
	lr2 := toe(0.2104542553*l2 + 0.7936177850*m2 - 0.0040720468*s2)

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
	cr1 := (float64(r1)*af1 + rb*(255-af1)) / 255
	cg1 := (float64(g1)*af1 + gb*(255-af1)) / 255
	cb1 := (float64(b1)*af1 + bb*(255-af1)) / 255
	cr2 := (float64(r2)*af2 + rb*(255-af2)) / 255
	cg2 := (float64(g2)*af2 + gb*(255-af2)) / 255
	cb2 := (float64(b2)*af2 + bb*(255-af2)) / 255

	lr1, lg1, lb1 := linLUT(cr1), linLUT(cg1), linLUT(cb1)
	lr2, lg2, lb2 := linLUT(cr2), linLUT(cg2), linLUT(cb2)

	l1 := cbrtLUT(0.4122214708*lr1 + 0.5363325363*lg1 + 0.0514459929*lb1)
	m1 := cbrtLUT(0.2119034982*lr1 + 0.6806995451*lg1 + 0.1073969566*lb1)
	s1 := cbrtLUT(0.0883024619*lr1 + 0.2817188376*lg1 + 0.6299787005*lb1)
	l2 := cbrtLUT(0.4122214708*lr2 + 0.5363325363*lg2 + 0.0514459929*lb2)
	m2 := cbrtLUT(0.2119034982*lr2 + 0.6806995451*lg2 + 0.1073969566*lb2)
	s2 := cbrtLUT(0.0883024619*lr2 + 0.2817188376*lg2 + 0.6299787005*lb2)

	lrA := toe(0.2104542553*l1 + 0.7936177850*m1 - 0.0040720468*s1)
	lrB := toe(0.2104542553*l2 + 0.7936177850*m2 - 0.0040720468*s2)

	return oklabHyabDelta(lrA-lrB, l1-l2, m1-m2, s1-s2, maxDelta)
}

// oklabHyabDelta compares the HyAB distance against maxDelta without taking
// a square root: the distance stays below the threshold exactly when
// |ΔLr| <= maxDelta and Δa² + Δb² <= (maxDelta − |ΔLr|)².
//
// Only the threshold test and the lighter/darker sign are ever used, so the
// result is reported as 0 or ±1 rather than an actual distance.
func oklabHyabDelta(dLr, dl, dm, ds, maxDelta float64) int {
	if rest := maxDelta - math.Abs(dLr); rest > 0 {
		da := 1.9779984951*dl - 2.4285922050*dm + 0.4505937099*ds
		db := 0.0259040371*dl + 0.7827717662*dm - 0.8086757660*ds
		if da*da+db*db <= rest*rest {
			return 0
		}
	}
	if dLr > 0 {
		return -1
	}
	return 1
}

// checkerboardBackground returns the RGB background color for a
// semi-transparent pixel at byte offset k. Each channel is either 48 or 207,
// producing a tri-tone noisy background that breaks alpha symmetries.
func checkerboardBackground(k int) (rb, gb, bb float64) {
	rb = 48 + 159*float64(k%2)
	// `(k / 1.618...) | 0` in JS truncates toward zero (works because k≥0).
	gb = 48 + 159*float64(int(float64(k)/1.618033988749895)%2)
	bb = 48 + 159*float64(int(float64(k)/2.618033988749895)%2)
	return
}
