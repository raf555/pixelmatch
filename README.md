# pixelmatch

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch?tab=doc)

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
— the smallest, simplest, fastest pixel-level image comparison library.
Pure Go, zero external dependencies, no cgo.

Features accurate **anti-aliased pixel detection** and **perceptual color
difference metrics** (YIQ NTSC color space, per Kotsarenko & Ramos 2010).

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
| `WithOutput(out)` | unset | write visual diff into the given `*image.NRGBA` |

## Image type handling

`Compare` and `CompareToImage` accept any `image.Image`:

- **`*image.NRGBA`** with tight stride and zero origin: **zero-copy** fast
  path. This is the recommended type — it's the format pixelmatch uses
  natively (straight, non-premultiplied RGBA).
- **`*image.RGBA`**: converted by un-premultiplying alpha. ~8-10% slower.
- **anything else** (`Gray`, `Paletted`, `YCbCr`, etc.): handled via
  `draw.Draw` to a temporary NRGBA. Always correct, slower.

## Performance

### Benchmark Results Summary

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
| **`CompareNRGBA800x600`** | 15.99ms ± 3% | 229.0MiB ± 3% | 0.000 ± 0% | 0.000 ± 0% |
| **`CompareNoOutputNRGBA`** | 9.230ms ± 4% | 396.8MiB ± 4% | 0.000 ± 0% | 0.000 ± 0% |
| **`CompareRGBA800x600`** | 17.66ms ± 3% | 207.3MiB ± 3% | 3.672MiB ± 0% | 2.000 ± 0% |

p.s. that 1 allocation comes from the options handling.

## Correctness

The port is verified byte-for-byte against the reference JavaScript
implementation across 14 test cases covering random images, gradient edges,
semi-transparency (both checkerboard and white-background modes), diff
masks, custom colors, stripe patterns, single-pixel images, and degenerate
aspect ratios. See the test files.

## License

ISC, same as the original mapbox/pixelmatch.
