# pixelmatch

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch/v8?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch/v8?tab=doc)

A native Go port of [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch),
the smallest, simplest, fastest pixel-level image comparison library.
Pure Go, no external dependencies, no cgo.

It detects **anti-aliased pixels** and measures **perceptual color
difference** in the OKLab color space (Ottosson 2020) with the HyAB metric
(Abasi et al. 2019).

## Install

```
go get github.com/raf555/pixelmatch/v8@latest
```

To stay on the previous YIQ-based major, keep the old import path
(`github.com/raf555/pixelmatch`) and pin a `v1.x` tag. You can import both
versions in the same program. See the [migration guide](#migration-guide)
when you're ready to move.

## Usage

```go
a, _ := png.Decode(fileA)
b, _ := png.Decode(fileB)

n, err := pixelmatch.Compare(a, b) // number of mismatched pixels
```

See the [package documentation](https://pkg.go.dev/github.com/raf555/pixelmatch/v8)
for diff images, the full option list, windowed diff counts and supported
image types.

## Performance

To measure on your own hardware:

```sh
go test -run='^$' -bench=. -benchmem -count=10 -cpu=1 .
```

## Correctness

The test suite checks this port byte for byte against the reference
JavaScript implementation on 25 generated cases. They cover random images,
gradient edges, semi-transparency over both the checkerboard and white
backgrounds, diff masks, custom colors, stripe patterns, single-pixel images,
extreme aspect ratios, near-black ramps for the Lr toe correction, saturated
hue pairs, the alpha tiebreak in the anti-aliasing detector, and the
windowed-count modes. The suite also runs the 14 upstream PNG fixture
comparisons and asserts both the diff count and byte-identical diff output.

## Differences from upstream

The comparison itself matches upstream byte for byte. These points differ
from the JavaScript version:

- **`*image.RGBA` input loses precision.** Upstream reads raw straight-alpha
  bytes. Go's `image.RGBA` stores premultiplied color, so a pixel with low
  alpha has already lost most of its color before `Compare` sees it. At the
  default threshold (0.1), a sweep of semi-transparent colors found no count
  changes, but at threshold 0 thousands of pixels flipped. Pass
  `*image.NRGBA` for exact results. `png.Decode` returns `*image.NRGBA` for
  PNGs with an alpha channel.
- **Other image types go through Go's color conversion.** Paletted, gray,
  16-bit and YCbCr images pass through `draw.Draw` into 8-bit NRGBA. A
  JavaScript PNG decoder may reduce the same file to slightly different
  bytes.
- **Empty images return an error.** Upstream returns 0 for a 0×0 image.
  `Compare` returns `ErrInvalidDimensions`. Go also returns errors where
  upstream throws.
- **`WithWindowSize` treats 0 and negatives as off.** Upstream treats any
  finite `windowSize`, including 0 and negatives, as a 1×1 window. Use
  `WithWindowSize(1)` for a 1×1 window. The option takes an `int`, so the
  upstream rules for fractional and `NaN` sizes don't apply.
- **Window counts don't wrap.** Upstream keeps per-column window counts in a
  `Uint16Array`, which wraps past 65,535 when the window is taller than
  65,535 pixels. This port uses `int32`.
- **No shared color cache.** Upstream memoizes converted colors in a
  module-level cache. This port leaves it out so you can call `Compare` from
  several goroutines at once. Results don't change, only speed.
- **Lookup tables match to within 1 ULP.** Both versions build the
  sRGB-to-linear and cube-root tables at startup. Go's `math.Pow` and
  `math.Cbrt` differ from V8's by 1 ULP on some entries. A 14.7M-pixel
  cross-check across seven thresholds and both blending modes found zero
  differing output bytes, but that parity rests on testing. Embedding the
  reference tables verbatim (~80KB of source) would guarantee it.
- **Results match across CPU architectures.** Go may fuse `x*y + z` into one
  FMA instruction on arm64 and other platforms, which skips a rounding step
  JavaScript performs. The color math blocks fusion, so amd64 and arm64
  produce the same bytes.

## Changelog

<details>
<summary><b>v8.0.0</b>: tracks upstream mapbox/pixelmatch 8.0.0</summary>

**Breaking**

- The module path moves to `github.com/raf555/pixelmatch/v8`.
- The color metric switches from YIQ to OKLab + HyAB with Ottosson's
  toe-corrected lightness. Diff counts and diff images change for the same
  inputs.
- The threshold now sets the maximum OKLab HyAB distance, with black to white
  at 1.0.
- Checkerboard blending picks each pixel's background from a Fibonacci hash
  of its index (8 colors) instead of the old tri-tone pattern.
- The anti-aliasing detector blends semi-transparent pixels over white
  instead of the checkerboard, and uses the alpha difference to break ties
  when luma cancels out.

**Added**

- `WithWindowSize(n)` returns the densest n×n diff count instead of the
  total.
- `WithIgnoreMask(m)` skips pixels marked in a one-byte-per-pixel mask.
- `ErrIgnoreMaskSizeMismatch` reports a mask with the wrong length.

**Fixed**

- Near-black differences no longer get over-weighted by cube-root table
  interpolation.

**Internal**

- Tests replay 25 cases generated from upstream's JavaScript
  (`internal/testutil/gen/generate.mjs`) and the 14 upstream fixture
  comparisons, checking output byte for byte.
- Products in the color math stay unfused, so results match on amd64, arm64
  and other architectures.

</details>

<details>
<summary><b>v1.0.1</b> (2026-05-17)</summary>

- Small performance optimization.
- README updates.

</details>

<details>
<summary><b>v1.0.0</b> (2026-05-15)</summary>

- First release: a Go port of mapbox/pixelmatch 7.x with the YIQ color
  metric.

</details>

## Migration guide

<details>
<summary><b>Upgrading from v1.x (upstream 7.x) to v8</b></summary>

v8 tracks upstream mapbox/pixelmatch 8.0.0, which swaps the YIQ color metric
for OKLab + HyAB. If you aren't ready to migrate, v1.x stays at the old
import path.

**1. Update the import path**

```sh
go get github.com/raf555/pixelmatch/v8@latest
```

```diff
-import "github.com/raf555/pixelmatch"
+import "github.com/raf555/pixelmatch/v8"
```

The package name stays `pixelmatch`. `Compare`, `CompareToImage` and the
existing `With*` options keep their signatures, so your calls compile
unchanged.

**2. Re-baseline your expectations**

- **Diff counts shift.** The same threshold now feeds a different metric, so
  counts won't match v1.x. Update your snapshot assertions.
- **Diff PNGs change byte for byte.** Regenerate your golden images.
- **The threshold scale stays 0..1**, with 1.0 as the black-to-white
  distance. A value tuned to sit right at the edge under YIQ may now pass or
  fail different pixels, so re-tune it if your tests depend on that edge.
- **Anti-aliasing results move at the edges.** Upstream fixed several cases
  around near-black and transparency-only differences. A few pixels v1.x
  flagged may now count as anti-aliasing, and the reverse.

**3. Consider the new options**

- `WithWindowSize(n)` makes tests fail on dense clusters of change instead of
  scattered noise. See [`WithWindowSize`](https://pkg.go.dev/github.com/raf555/pixelmatch/v8#WithWindowSize).
- `WithIgnoreMask(m)` skips regions such as timestamps or animated UI.

</details>

## License

ISC, same as the original mapbox/pixelmatch.
