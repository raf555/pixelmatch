# pixelmatch

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch/v7?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch/v7?tab=doc)

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
7.2.0, a small, fast pixel-level image comparison library. It's pure Go, with
no dependencies and no cgo.

It detects **anti-aliased pixels** and measures **perceptual color
difference** in the YIQ NTSC color space (Kotsarenko & Ramos 2010).

This is the v7 line, which tracks upstream 7.x. For upstream 8.x (OKLab +
HyAB), use [`github.com/raf555/pixelmatch/v8`](https://pkg.go.dev/github.com/raf555/pixelmatch/v8).

## Install

```
go get github.com/raf555/pixelmatch/v7@latest
```

## Usage

Count differing pixels:

```go
import (
    "image/png"

    "github.com/raf555/pixelmatch/v7"
)

a, _ := png.Decode(fileA)
b, _ := png.Decode(fileB)

n, err := pixelmatch.Compare(a, b) // number of mismatched pixels
```

Write a visual diff with `WithOutput`:

```go
out := image.NewNRGBA(image.Rect(0, 0, w, h))
n, err := pixelmatch.Compare(a, b,
    pixelmatch.WithThreshold(0.1),
    pixelmatch.WithOutput(out),
    pixelmatch.WithDiffColor(255, 0, 255),
)
_ = png.Encode(file, out)
```

Or get a freshly allocated diff in one call:

```go
diff, n, err := pixelmatch.CompareToImage(a, b) // diff is a *image.NRGBA
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

- **`*image.NRGBA`** with tight stride and zero origin takes a **zero-copy**
  path. Pass this type when you can: pixelmatch works on straight,
  non-premultiplied RGBA.
- **`*image.RGBA`** gets un-premultiplied first, about 10% slower.
- **Other types** (`Gray`, `Paletted`, `YCbCr` and so on) go through
  `draw.Draw` into a temporary NRGBA. Correct, but slower.

## Performance

```sh
go test -bench=. -benchmem -count=10 -cpu 1
```

Measured on linux/amd64, AMD EPYC 9634 84-Core Processor:

| Benchmark | Time (`sec/op`) | Throughput (`B/s`) | Memory (`B/op`) | Allocations (`allocs/op`) |
| :--- | :--- | :--- | :--- | :--- |
| **`CompareNRGBA800x600`** | 16.05ms ± 3% | 228.2MiB ± 3% | 48.00B ± 0% | 1.000 ± 0% |
| **`CompareNoOutputNRGBA`** | 9.137ms ± 3% | 400.8MiB ± 3% | 48.00B ± 0% | 1.000 ± 0% |
| **`CompareRGBA800x600`** | 17.76ms ± 3% | 206.2MiB ± 4% | 3.672MiB ± 0% | 3.000 ± 0% |

The single allocation comes from option handling.

## Correctness

The tests replay 18 cases generated from upstream's JavaScript
(`internal/testutil/gen/generate.mjs`) and the upstream 7.2.0 PNG fixtures,
checking diff counts and diff images byte for byte. The cases cover random
images, gradient edges, semi-transparency over checkerboard and white, diff
masks, custom colors, stripes, single-pixel images and extreme aspect ratios.

CI runs the suite on Linux, macOS (arm64) and Windows, and again on amd64
with FMA instructions enabled (`GOAMD64=v3`).

## Changelog

<details>
<summary><b>v7.0.0</b>: tracks upstream mapbox/pixelmatch 7.2.0</summary>

**Breaking**

- The module path moves from `github.com/raf555/pixelmatch` to
  `github.com/raf555/pixelmatch/v7`, so the major version matches upstream.

**Fixed**

- The color math blocks FMA fusion, so arm64 (Apple Silicon included)
  produces the same results as amd64 and upstream. Before, Go fused
  multiply-adds on arm64 and rounding could differ.

**Internal**

- Tests replay 18 cases generated from upstream 7.2.0's JavaScript and the
  upstream fixtures, checking output byte for byte. Fixture diffs now get a
  byte-equality check on top of the count.
- CI runs lint and tests on Linux, macOS and Windows.

</details>

<details>
<summary><b>v1.0.1</b> (2026-05-17)</summary>

- Reordered the internal `Options` fields to shrink the struct.
- Added tests and README updates.

</details>

<details>
<summary><b>v1.0.0</b> (2026-05-15)</summary>

- First release: a Go port of mapbox/pixelmatch 7.2.0 with the YIQ color
  metric.

</details>

## Migrating from v1.x

v1.x and v7 share the same code. Change the import path:

```sh
go get github.com/raf555/pixelmatch/v7@latest
```

```diff
-import "github.com/raf555/pixelmatch"
+import "github.com/raf555/pixelmatch/v7"
```

Diff counts and diff images stay identical on amd64. On arm64, rare pixels
near the threshold may change, now matching upstream.

## License

ISC, same as the original mapbox/pixelmatch.
