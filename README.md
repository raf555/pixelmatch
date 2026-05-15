# pixelmatch-go

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
v7.2.0 — the smallest, simplest, fastest pixel-level image comparison library.
Pure Go, zero external dependencies, no cgo.

Features accurate **anti-aliased pixel detection** and **perceptual color
difference metrics** (YIQ NTSC color space, per Kotsarenko & Ramos 2010).

## Install

```
go get github.com/raf555/pixelmatch
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

n, err := pixelmatch.Compare(a, b, pixelmatch.WithThreshold(0.1))
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
png.Encode(file, out)
```

Or get a freshly allocated diff in one call:

```go
diff, n, err := pixelmatch.CompareToImage(a, b, pixelmatch.WithThreshold(0.1))
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
| `WithOutput(out)` | unset | write visual diff into the given `*image.NRGBA` |

## Image type handling

`Compare` and `CompareToImage` accept any `image.Image`:

- **`*image.NRGBA`** with tight stride and zero origin: **zero-copy** fast
  path. This is the recommended type — it's the format pixelmatch uses
  natively (straight, non-premultiplied RGBA).
- **`*image.RGBA`**: converted by un-premultiplying alpha. ~24% slower.
- **anything else** (`Gray`, `Paletted`, `YCbCr`, etc.): handled via
  `draw.Draw` to a temporary NRGBA. Always correct, slower.

`png.Decode` returns whichever concrete type matches the source bit depth
and alpha. The wrapper handles all of them.

## Performance

On a Xeon @ 2.10 GHz, comparing two 800×600 images:

```
BenchmarkMatch800x600           ~17.8 ms/op   216 MB/s   0 allocs (byte API)
BenchmarkMatchIdentical800x600  ~10.6 ms/op   362 MB/s   0 allocs
BenchmarkCompareNRGBA800x600    ~18.4 ms/op   208 MB/s   1 alloc  (image.Image API, NRGBA)
BenchmarkCompareRGBA800x600     ~22.5 ms/op   170 MB/s   3 allocs (with un-premultiply)
```

The `image.Image` API is essentially free over the byte API when the input
is `*image.NRGBA`.

## Correctness

The port is verified byte-for-byte against the reference JavaScript
implementation across 14 test cases covering random images, gradient edges,
semi-transparency (both checkerboard and white-background modes), diff
masks, custom colors, stripe patterns, single-pixel images, and degenerate
aspect ratios. See `pixelmatch_test.go`, `cross_test.go`, and
`image_test.go`.
