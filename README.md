# pixelmatch

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch/v8?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch/v8?tab=doc)

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
— the smallest, simplest, fastest pixel-level image comparison library.
Pure Go, zero external dependencies, no cgo.

Features accurate **anti-aliased pixel detection** and **perceptual color
difference metrics** (OKLab color space per Ottosson 2020, compared with the
HyAB metric of Abasi et al. 2019).

## Install

```
go get github.com/raf555/pixelmatch/v8@latest
```

Still on the previous major (YIQ-based)? Keep the old import path
(`github.com/raf555/pixelmatch`) and pin to a `v1.x` tag — the two can
coexist in the same program.

## Usage

The simplest path — count differing pixels:

```go
import (
    "image/png"
    "os"
    "github.com/raf555/pixelmatch/v8"
)

a, _ := png.Decode(fileA)
b, _ := png.Decode(fileB)

n, err := pixelmatch.Compare(a, b)
// n = number of mismatched pixels
```

Add a visual diff with `WithOutput`:

```go
out := image.NewNRGBA(image.Rect(0, 0, w, h))
n, err := pixelmatch.Compare(a, b,
    pixelmatch.WithThreshold(0.1),
    pixelmatch.WithOutput(out),
    pixelmatch.WithDiffColor(255, 0, 255),
)
// out now contains the visual diff
_ = png.Encode(file, out)
```

Or get a freshly allocated diff in one call:

```go
diff, n, err := pixelmatch.CompareToImage(a, b)
// diff is a *image.NRGBA
```

## Upgrading from v7.x (v1.x of this port)

v8 tracks upstream mapbox/pixelmatch 8.0.0, which replaces the YIQ color
metric with OKLab + HyAB. The import path moves to
`github.com/raf555/pixelmatch/v8`; v1.x stays available at the old path for
anyone not ready to migrate.

What to expect when upgrading:

- **Diff counts shift.** Same threshold, different metric — numbers won't
  line up with v1.x. Re-baseline any snapshot assertions.
- **Diff PNGs change byte-for-byte.** Golden-image CI will need
  regeneration.
- **Threshold scale is preserved.** It's still 0..1, where 1.0 is black vs
  white. But values that were "just tight enough" against YIQ may be too
  loose or too strict against HyAB — re-tune if accuracy matters.
- **Public API is source-compatible.** `Compare`, `CompareToImage`, and
  every `With*` option keep the same signatures. `WithWindowSize` and
  `WithIgnoreMask` are new.
- **Near-black and transparency-only cases behave better.** Upstream fixed
  a handful of AA edge cases here; a few pixels v1.x flagged may now be
  excluded and vice versa.

## Options

| Option | Default | Meaning |
|---|---|---|
| `WithThreshold(0.1)` | `0.1` | matching threshold (0..1); smaller = more sensitive |
| `WithIncludeAA(false)` | `false` | if true, count anti-aliased pixels as differences |
| `WithAlpha(0.1)` | `0.1` | opacity of the original image in the diff output |
| `WithAAColor(255,255,0)` | yellow | color for AA pixels |
| `WithDiffColor(255,0,0)` | red | color for diff pixels |
| `WithDiffColorAlt(r,g,b)` | unset | alt color for darker-in-img2 pixels |
| `WithDiffMask(false)` | `false` | draw diff over transparent background |
| `WithCheckerboard(true)` | `true` | blend semi-transparent pixels against checkerboard |
| `WithWindowSize(n)` | unset | report the densest n×n local diff count instead of the total |
| `WithIgnoreMask(m)` | unset | skip pixels where `m[i] != 0` (one byte per pixel, length `w*h`) |
| `WithOutput(out)` | unset | write visual diff into the given `*image.NRGBA` |

The threshold is the maximum acceptable OKLab HyAB distance between two
colors, normalized so that black-to-white is exactly `1.0`.

## Windowed diff density

By default `Compare` returns the total number of differing pixels. With
`WithWindowSize(n)` it instead returns the largest number of diff pixels
found in any n×n region (`n` is clamped to the image dimensions, and
anti-aliased pixels are never counted):

```go
n, err := pixelmatch.Compare(a, b, pixelmatch.WithWindowSize(32))
if float64(n)/(32*32) > 0.05 {
    // a dense cluster of differences — a real regression
}
```

This makes the result robust to scattered noise. Spread-out speckle from GPU
dithering or sub-pixel anti-aliasing never packs densely into a single
window, while a genuine regression does. Failing on density rather than
raw count stays comparable across image sizes, so you can run a stricter
threshold without tripping over noise. The total count is just the
degenerate whole-image window.

The diff image, if requested, is unaffected — it still marks every differing
pixel.

## Image type handling

`Compare` and `CompareToImage` accept any `image.Image`:

- **`*image.NRGBA`** with tight stride and zero origin: **zero-copy** fast
  path. This is the recommended type — it's the format pixelmatch uses
  natively (straight, non-premultiplied RGBA).
- **`*image.RGBA`**: converted by un-premultiplying alpha into a temporary
  buffer. Slower, and allocates.
- **anything else** (`Gray`, `Paletted`, `YCbCr`, etc.): handled via
  `draw.Draw` to a temporary NRGBA. Always correct, slower.

## Performance

To measure on your own hardware:

```sh
go test -run='^$' -bench=. -benchmem -count=10 -cpu=1 .
```

## Correctness

The port is verified byte-for-byte against the reference JavaScript
implementation across 25 generated test cases covering random images,
gradient edges, semi-transparency (both checkerboard and white-background
modes), diff masks, custom colors, stripe patterns, single-pixel images,
degenerate aspect ratios, near-black ramps that exercise the Lr toe
correction, saturated hue pairs, the alpha-tiebreak path in the anti-aliasing detector, and every windowed-count mode. It is
additionally checked against all 14 upstream PNG fixture comparisons, asserting both the
expected diff count and byte-identical diff output. See the test files.

One caveat worth knowing: the reference implementation approximates
sRGB-to-linear conversion and the cube root with interpolated lookup tables,
which this port reproduces. Go's `math.Pow` and `math.Cbrt` disagree with
V8's by 1 ULP on some table entries, so the tables are not bit-identical
even though every value is within one ULP. A 14.7M-pixel cross-check across
seven thresholds and both blending modes produced zero differing output
bytes, but exact parity is empirical here rather than structural. Embedding
the reference tables verbatim would make it structural, at the cost of ~80KB
of generated source.

## License

ISC, same as the original mapbox/pixelmatch.
