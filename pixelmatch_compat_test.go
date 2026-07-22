package pixelmatch

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCompatiblity(t *testing.T) {
	tcs := []struct {
		img1, img2, diff string
		expectedMismatch int
		opts             []Option
	}{
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diff",
			expectedMismatch: 154,
			opts:             []Option{WithThreshold(0.05)},
		},
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diffdefaultthreshold",
			expectedMismatch: 120,
		},
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diffmask",
			expectedMismatch: 154,
			opts: []Option{
				WithThreshold(0.05),
				WithDiffMask(true),
			},
		},
		{
			img1:             "1a",
			img2:             "1a",
			diff:             "1emptydiffmask",
			expectedMismatch: 0,
			opts: []Option{
				WithThreshold(0),
				WithDiffMask(true),
			},
		},
		{
			img1:             "2a",
			img2:             "2b",
			diff:             "2diff",
			expectedMismatch: 12821,
			opts: []Option{
				WithThreshold(0.05),
				WithAlpha(0.5),
				WithAAColor(0, 192, 0),
				WithDiffColor(255, 0, 255),
			},
		},
		{
			img1:             "3a",
			img2:             "3b",
			diff:             "3diff",
			expectedMismatch: 220,
			opts:             []Option{WithThreshold(0.05)},
		},
		{
			img1:             "4a",
			img2:             "4b",
			diff:             "4diff",
			expectedMismatch: 36563,
			opts:             []Option{WithThreshold(0.05)},
		},
		{
			img1:             "5a",
			img2:             "5b",
			diff:             "5diff",
			expectedMismatch: 0,
			opts:             []Option{WithThreshold(0.05)},
		},
		{
			img1:             "6a",
			img2:             "6b",
			diff:             "6diff",
			expectedMismatch: 51,
			opts:             []Option{WithThreshold(0.05)},
		},
		{
			img1:             "6a",
			img2:             "6a",
			diff:             "6empty",
			expectedMismatch: 0,
			opts:             []Option{WithThreshold(0)},
		},
		{
			img1:             "7a",
			img2:             "7b",
			diff:             "7diff",
			expectedMismatch: 2440,
			opts:             []Option{WithDiffColorAlt(0, 255, 0)},
		},
		{
			img1:             "8a",
			img2:             "5b",
			diff:             "8diff",
			expectedMismatch: 32896,
			opts:             []Option{WithThreshold(0.05)},
		},
	}

	for _, tc := range tcs {
		name, test := diffTest(tc.img1, tc.img2, tc.diff, tc.expectedMismatch, tc.opts...)
		t.Run(name, test)
	}
}

const basePath = "internal/testutil/data"

func mustReadImage(t testing.TB, imageName string) image.Image {
	f, err := os.Open(filepath.Join(basePath, imageName+".png"))
	if err != nil {
		t.Fatalf("mustReadImage: failed to open test image: %s", err.Error())
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("mustReadImage: failed to decode image: %s", err.Error())
	}

	return img
}

func TestCheckerboard(t *testing.T) {
	img1 := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img1.Pix = []byte{0, 0, 0, 128} // 50% transparent black

	img2 := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img2.Pix = []byte{127, 127, 127, 255} // opaque gray

	n, err := Compare(img1, img2, WithCheckerboard(false))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("checkerboard=false: got %d diffs, want 0", n)
	}

	n, err = Compare(img1, img2)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("checkerboard=true (default): got %d diffs, want 1", n)
	}
}

// TestWindowSizeOnFixture mirrors the reference suite's windowed test on a
// real 256x256 fixture pair holding 51 differing pixels.
func TestWindowSizeOnFixture(t *testing.T) {
	img1 := mustReadImage(t, "6a")
	img2 := mustReadImage(t, "6b")

	total, err := Compare(img1, img2, WithThreshold(0.05))
	if err != nil {
		t.Fatal(err)
	}
	if total != 51 {
		t.Fatalf("total diff count = %d, want 51", total)
	}

	tests := []struct {
		windowSize int
		want       int
	}{
		// A window at least as large as the image is the degenerate
		// whole-image window, so it must equal the total.
		{256, total},
		{100000, total},
		// Smaller windows hold a subset of the diffs, never more.
		{32, 29},
		{8, 6},
	}

	for _, tc := range tests {
		got, err := Compare(img1, img2, WithThreshold(0.05), WithWindowSize(tc.windowSize))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("windowSize %d: got %d, want %d", tc.windowSize, got, tc.want)
		}
	}
}

func diffTest(imgPath1, imgPath2, diffImgPath string, expectedMismatch int, opts ...Option) (string, func(*testing.T)) {
	name := fmt.Sprintf("comparing %s to %s against %s", imgPath1, imgPath2, diffImgPath)

	return name, func(t *testing.T) {
		t.Parallel()

		img1 := mustReadImage(t, imgPath1)
		img2 := mustReadImage(t, imgPath2)

		diffNRGBA, mismatch, err := CompareToImage(img1, img2, opts...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on comparing image 1 and 2 with output diff: %s", err.Error())
		}

		mismatch2, err := Compare(img1, img2, opts...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on comparing image 1 and 2 without output diff: %s", err.Error())
		}

		expectedDiff := mustReadImage(t, diffImgPath)
		mismatch3, err := Compare(expectedDiff, diffNRGBA, opts...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on comparing expected diff and output diff: %s", err.Error())
		}

		// The comparison above runs at the case's own threshold, so it would
		// tolerate small per-pixel drift. Assert byte equality as well.
		wantPix := image.NewNRGBA(expectedDiff.Bounds())
		draw.Draw(wantPix, wantPix.Bounds(), expectedDiff, expectedDiff.Bounds().Min, draw.Src)
		if !bytes.Equal(diffNRGBA.Pix, wantPix.Pix) {
			bad := 0
			for i := range diffNRGBA.Pix {
				if diffNRGBA.Pix[i] != wantPix.Pix[i] {
					bad++
				}
			}
			t.Errorf("diffTest: diff output is not byte-identical to the fixture: %d of %d bytes differ",
				bad, len(wantPix.Pix))
		}

		if mismatch != expectedMismatch {
			t.Errorf("diffTest: mismatch = %d, want = %d", mismatch, expectedMismatch)
		}

		if mismatch != mismatch2 {
			t.Errorf("diffTest: mismatch with vs without diff output = %d, want = %d", mismatch2, mismatch)
		}

		if mismatch3 != 0 {
			t.Errorf("diffTest: diff output mismatch = %d, want = 0", mismatch3)
		}
	}
}
