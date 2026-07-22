# pixelmatch

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch?tab=doc)

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
— the smallest, simplest, fastest pixel-level image comparison library.
Pure Go, zero external dependencies, no cgo.

Features accurate **anti-aliased pixel detection** and **perceptual color
difference metrics** (OKLab color space per Ottosson 2020, compared with the
HyAB metric of Abasi et al. 2019).

## Install

```
go get github.com/raf555/pixelmatch@latest
```

## Usage

The simplest path — count differing pixels:

```go
import (
    "image/png"
    "os"
    "github.com/raf555/pixelmatch"
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
- **`*image.RGBA`**: converted by un-premultiplying alpha. ~10% slower.
- **anything else** (`Gray`, `Paletted`, `YCbCr`, etc.): handled via
  `draw.Draw` to a temporary NRGBA. Always correct, slower.

## Performance

### Benchmark Results Summary

Figures below were measured against the previous YIQ implementation. In a
same-machine re-run the OKLab switch came out neutral to marginally faster,
but these numbers predate it and are worth re-measuring on your hardware.

**Command**

```sh
go test -bench=. -benchmem -count=10 -cpu 1
```

**Environment:**
* **OS/Arch:** linux/amd64
* **Package:** github.com/raf555/pixelmatch
* **CPU:** AMD EPYC 9634 84-Core Processor

| Benchmark | Time (`sec/op`) | Throughput (`B/s`) | Memory (`B/op`) | Allocations (`allocs/op`) |
| :--- | :--- | :--- | :--- | :--- |
| **`CompareNRGBA800x600`** | 16.05ms ± 3% | 228.2MiB ± 3% | 48.00B ± 0% | 1.000 ± 0% |
| **`CompareNoOutputNRGBA`** | 9.137ms ± 3% | 400.8MiB ± 3% | 48.00B ± 0% | 1.000 ± 0% |
| **`CompareRGBA800x600`** | 17.76ms ± 3% | 206.2MiB ± 4% | 3.672MiB ± 0% | 3.000 ± 0% |

*p.s. that 1 allocation comes from the options handling.*

## Correctness

The port is verified byte-for-byte against the reference JavaScript
implementation across 25 generated test cases covering random images,
gradient edges, semi-transparency (both checkerboard and white-background
modes), diff masks, custom colors, stripe patterns, single-pixel images,
degenerate aspect ratios, near-black ramps that exercise the Lr toe
correction, saturated hue pairs, the alpha-tiebreak path in the anti-aliasing detector, and every windowed-count mode. It is
additionally checked against all 12 upstream PNG fixtures, asserting both the
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
