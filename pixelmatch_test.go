package pixelmatch

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/raf555/pixelmatch/internal/testutil"
)

func TestCompareIdentical(t *testing.T) {
	w, h := 16, 16
	pix := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return uint8(x * 16), uint8(y * 16), 100, 255
	})
	img1 := testutil.BytesToNRGBA(pix, w, h)
	img2 := testutil.BytesToNRGBA(pix, w, h)
	out := image.NewNRGBA(image.Rect(0, 0, w, h))

	n, err := Compare(img1, img2, WithOutput(out))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}

func TestCompareNoOutput(t *testing.T) {
	w, h := 8, 8
	img1 := image.NewNRGBA(image.Rect(0, 0, w, h))
	img2 := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Make img2 fully white, img1 fully zero — every pixel should differ.
	for i := range img2.Pix {
		img2.Pix[i] = 255
	}
	n, err := Compare(img1, img2)
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("got %d, want %d", n, w*h)
	}
}

func TestCompareDimensionMismatch(t *testing.T) {
	img1 := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewNRGBA(image.Rect(0, 0, 11, 10))
	if _, err := Compare(img1, img2); err == nil {
		t.Error("expected dimension mismatch error")
	}

	out := image.NewNRGBA(image.Rect(0, 0, 9, 9))
	img3 := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if _, err := Compare(img1, img3, WithOutput(out)); err == nil {
		t.Error("expected output dimension mismatch error")
	}
}

func TestCompareNilInputs(t *testing.T) {
	if _, err := Compare(nil, nil); err == nil {
		t.Error("expected nil-input error")
	}
}

func TestCompareToImageReturnsDiff(t *testing.T) {
	w, h := 4, 4
	img1 := image.NewNRGBA(image.Rect(0, 0, w, h))
	img2 := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img1.Pix {
		img1.Pix[i] = 0
	}
	for i := range img2.Pix {
		img2.Pix[i] = 200
	}
	diff, n, err := CompareToImage(img1, img2)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil {
		t.Fatal("diff is nil")
	}
	if n != w*h {
		t.Errorf("got %d diffs, want %d", n, w*h)
	}
	if diff.Bounds().Dx() != w || diff.Bounds().Dy() != h {
		t.Errorf("wrong diff bounds: %v", diff.Bounds())
	}
}

func TestFunctionalOptions(t *testing.T) {
	w, h := 4, 4
	img1 := image.NewNRGBA(image.Rect(0, 0, w, h))
	img2 := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img1.Pix {
		img1.Pix[i] = 100
	}
	for i := range img2.Pix {
		img2.Pix[i] = 105 // very small difference
	}

	// Loose threshold → no diffs.
	n, err := Compare(img1, img2, WithThreshold(0.5))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("loose: got %d, want 0", n)
	}

	// Tight threshold → all pixels diff.
	n, err = Compare(img1, img2, WithThreshold(0.0))
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("tight: got %d, want %d", n, w*h)
	}
}

func TestWithDiffColorAlt(t *testing.T) {
	w, h := 2, 1
	img1 := image.NewNRGBA(image.Rect(0, 0, w, h))
	img2 := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Pixel 0: img1 light, img2 dark → img2 darker → alt color.
	img1.Set(0, 0, color.NRGBA{240, 240, 240, 255})
	img2.Set(0, 0, color.NRGBA{20, 20, 20, 255})
	// Pixel 1: img1 dark, img2 light → img2 lighter → diff color.
	img1.Set(1, 0, color.NRGBA{20, 20, 20, 255})
	img2.Set(1, 0, color.NRGBA{240, 240, 240, 255})

	diff, _, err := CompareToImage(img1, img2,
		WithDiffColor(255, 0, 0),
		WithDiffColorAlt(0, 0, 255),
	)
	if err != nil {
		t.Fatal(err)
	}

	p0 := diff.NRGBAAt(0, 0)
	if p0.R != 0 || p0.G != 0 || p0.B != 255 {
		t.Errorf("pixel 0: expected blue (alt), got %v", p0)
	}
	p1 := diff.NRGBAAt(1, 0)
	if p1.R != 255 || p1.G != 0 || p1.B != 0 {
		t.Errorf("pixel 1: expected red, got %v", p1)
	}
}

// TestAsStraightRGBAFastPath verifies that NRGBA inputs with the standard
// layout don't trigger a copy.
func TestAsStraightRGBAFastPath(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	pix := asStraightRGBA(img)
	// Aliasing test: modify the source, verify the returned slice sees it.
	img.Pix[0] = 99
	if pix[0] != 99 {
		t.Error("expected fast path to alias source NRGBA")
	}
}

// TestAsStraightRGBAFromRGBA verifies premultiplied → straight conversion.
func TestAsStraightRGBAFromRGBA(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	// In premultiplied space: 50% red on 50% alpha → R=128, A=128 stored.
	img.Set(0, 0, color.RGBA{128, 0, 0, 128})
	pix := asStraightRGBA(img)
	// After un-premultiply, R should be ~255 (full red at 50% alpha).
	// (128 * 255 / 128 = 255, exact).
	if pix[0] != 255 {
		t.Errorf("expected R=255 after un-premultiply, got %d", pix[0])
	}
	if pix[3] != 128 {
		t.Errorf("expected A=128 unchanged, got %d", pix[3])
	}
}

// TestAsStraightRGBAFromOther verifies the generic fallback (Gray here).
func TestAsStraightRGBAFromGray(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 2, 1))
	img.SetGray(0, 0, color.Gray{Y: 0})   // black
	img.SetGray(1, 0, color.Gray{Y: 255}) // white
	pix := asStraightRGBA(img)
	if len(pix) != 8 {
		t.Fatalf("wrong length: %d", len(pix))
	}
	// First pixel: black opaque.
	if pix[0] != 0 || pix[1] != 0 || pix[2] != 0 || pix[3] != 255 {
		t.Errorf("pixel 0: got [%d %d %d %d]", pix[0], pix[1], pix[2], pix[3])
	}
	// Second pixel: white opaque.
	if pix[4] != 255 || pix[5] != 255 || pix[6] != 255 || pix[7] != 255 {
		t.Errorf("pixel 1: got [%d %d %d %d]", pix[4], pix[5], pix[6], pix[7])
	}
}

// TestCompareWithNonTightOutput verifies that an NRGBA with a non-zero
// origin or non-tight stride still gets the right diff written to it.
func TestCompareWithNonTightOutput(t *testing.T) {
	w, h := 8, 8
	img1 := image.NewNRGBA(image.Rect(0, 0, w, h))
	img2 := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img2.Pix {
		img2.Pix[i] = 255 // all-white img2 vs all-zero img1
	}

	// Output with origin at (5, 5).
	bigOut := image.NewNRGBA(image.Rect(5, 5, 5+w, 5+h))
	n, err := Compare(img1, img2, WithOutput(bigOut))
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("got %d, want %d", n, w*h)
	}
	// Check the first diff pixel at the output's origin (5,5).
	p := bigOut.NRGBAAt(5, 5)
	if p.R != 255 || p.G != 0 || p.B != 0 || p.A != 255 {
		t.Errorf("expected red diff at origin, got %v", p)
	}
}

// TestCompareCrossValidatesAgainstByteAPI compares the high-level
// image.Image API against the low-level byte API for the JS reference
// cases, ensuring no precision loss in the wrapping.
func TestCompareCrossValidatesAgainstByteAPI(t *testing.T) {
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Skipf("no reference cases: %v", err)
	}
	var cases []refCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			img1Pix, _ := base64.StdEncoding.DecodeString(c.Img1)
			img2Pix, _ := base64.StdEncoding.DecodeString(c.Img2)

			// Build opts the same way the byte test does.
			var opts []Option
			if c.Opts.Threshold != nil {
				opts = append(opts, WithThreshold(*c.Opts.Threshold))
			}
			if c.Opts.IncludeAA != nil {
				opts = append(opts, WithIncludeAA(*c.Opts.IncludeAA))
			}
			if c.Opts.Alpha != nil {
				opts = append(opts, WithAlpha(*c.Opts.Alpha))
			}
			if c.Opts.DiffMask != nil {
				opts = append(opts, WithDiffMask(*c.Opts.DiffMask))
			}
			if c.Opts.Checkerboard != nil {
				opts = append(opts, WithCheckerboard(*c.Opts.Checkerboard))
			}
			if c.Opts.DiffColor != nil {
				opts = append(opts, WithDiffColor(
					uint8((*c.Opts.DiffColor)[0]),
					uint8((*c.Opts.DiffColor)[1]),
					uint8((*c.Opts.DiffColor)[2]),
				))
			}
			if c.Opts.DiffColorAlt != nil {
				opts = append(opts, WithDiffColorAlt(
					uint8((*c.Opts.DiffColorAlt)[0]),
					uint8((*c.Opts.DiffColorAlt)[1]),
					uint8((*c.Opts.DiffColorAlt)[2]),
				))
			}
			if c.Opts.AAColor != nil {
				opts = append(opts, WithAAColor(
					uint8((*c.Opts.AAColor)[0]),
					uint8((*c.Opts.AAColor)[1]),
					uint8((*c.Opts.AAColor)[2]),
				))
			}

			img1 := testutil.BytesToNRGBA(img1Pix, c.W, c.H)
			img2 := testutil.BytesToNRGBA(img2Pix, c.W, c.H)
			diff, n, err := CompareToImage(img1, img2, opts...)
			if err != nil {
				t.Fatal(err)
			}
			if n != c.N {
				t.Errorf("diff count: got %d, want %d", n, c.N)
			}
			wantDiff, _ := base64.StdEncoding.DecodeString(c.Diff)
			if !bytes.Equal(diff.Pix, wantDiff) {
				t.Errorf("diff buffer differs from JS reference")
			}
		})
	}
}

// TestRoundTripPNG sanity-checks that decoded PNGs go through Compare
// correctly.
func TestRoundTripPNG(t *testing.T) {
	w, h := 8, 8
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.NRGBA{uint8(x * 32), uint8(y * 32), 128, 255})
		}
	}
	// Encode and re-decode.
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Decoded is *image.RGBA (since src is opaque). Compare against the
	// original NRGBA — there should be zero differences.
	n, err := Compare(src, decoded)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("round-trip lost pixels: %d diffs", n)
	}
}

// TestDrawPackageContract ensures *image.NRGBA still satisfies draw.Image.
// (Doubles as a compile-time sanity check on the import graph.)
func TestDrawPackageContract(t *testing.T) {
	var _ draw.Image = image.NewNRGBA(image.Rect(0, 0, 1, 1))
}
