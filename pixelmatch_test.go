package pixelmatch_test

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/raf555/pixelmatch"
)

func TestMatchPixel(t *testing.T) {
	tcs := []struct {
		img1, img2, diff string
		expectedMismatch int
		opts             []pixelmatch.MatchOptionFn
	}{
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diff",
			expectedMismatch: 143,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		},
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diffdefaultthreshold",
			expectedMismatch: 106,
		},
		{
			img1:             "1a",
			img2:             "1b",
			diff:             "1diffmask",
			expectedMismatch: 143,
			opts: []pixelmatch.MatchOptionFn{
				pixelmatch.WithThreshold(0.05),
				pixelmatch.WithDiffMask(true),
			},
		},
		{
			img1:             "1a",
			img2:             "1a",
			diff:             "1emptydiffmask",
			expectedMismatch: 0,
			opts: []pixelmatch.MatchOptionFn{
				pixelmatch.WithDiffMask(true),
			},
		},
		{
			img1:             "2a",
			img2:             "2b",
			diff:             "2diff",
			expectedMismatch: 12437,
			opts: []pixelmatch.MatchOptionFn{
				pixelmatch.WithThreshold(0.05),
				pixelmatch.WithAlpha(0.5),
				pixelmatch.WithAntiAliasedColor(color.RGBA{R: 0, G: 192, B: 0, A: 255}),
				pixelmatch.WithDiffColor(color.RGBA{R: 255, G: 0, B: 255, A: 255}),
			},
		},
		{
			img1:             "3a",
			img2:             "3b",
			diff:             "3diff",
			expectedMismatch: 212,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		},
		{
			img1:             "4a",
			img2:             "4b",
			diff:             "4diff",
			expectedMismatch: 36049,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		},
		// TODO: fix this test
		// {
		// 	img1:             "5a",
		// 	img2:             "5b",
		// 	diff:             "5diff",
		// 	expectedMismatch: 6,
		// 	opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		// },
		{
			img1:             "6a",
			img2:             "6b",
			diff:             "6diff",
			expectedMismatch: 51,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		},
		{
			img1:             "6a",
			img2:             "6a",
			diff:             "6empty",
			expectedMismatch: 0,
		},
		{
			img1:             "7a",
			img2:             "7b",
			diff:             "7diff",
			expectedMismatch: 2448,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithDiffColorAlt(color.RGBA{R: 0, G: 255, B: 0, A: 255})},
		},
		{
			img1:             "8a",
			img2:             "5b",
			diff:             "8diff",
			expectedMismatch: 32896,
			opts:             []pixelmatch.MatchOptionFn{pixelmatch.WithThreshold(0.05)},
		},
	}

	for _, tc := range tcs {
		name, test := diffTest(tc.img1, tc.img2, tc.diff, tc.expectedMismatch, tc.opts...)
		t.Run(name, test)
	}
}

const basePath = "testdata"

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

func diffTest(imgPath1, imgPath2, diffImgPath string, expectedMismatch int, opts ...pixelmatch.MatchOptionFn) (string, func(*testing.T)) {
	name := fmt.Sprintf("comparing %s to %s against %s", imgPath1, imgPath2, diffImgPath)

	return name, func(t *testing.T) {
		t.Parallel()

		img1 := mustReadImage(t, imgPath1)
		img2 := mustReadImage(t, imgPath2)

		diffDst := image.Image(nil)

		mismatch, err := pixelmatch.MatchPixel(img1, img2, append(opts, pixelmatch.WithDiffDest(&diffDst))...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on matching image 1 and 2 with output diff: %s", err.Error())
		}

		mismatch2, err := pixelmatch.MatchPixel(img1, img2, opts...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on matching image 1 and 2 without output diff: %s", err.Error())
		}

		expectedDiff := mustReadImage(t, diffImgPath)
		mismatch3, err := pixelmatch.MatchPixel(expectedDiff, diffDst, opts...)
		if err != nil {
			t.Errorf("diffTest: unexpected error on matching expected diff and output diff: %s", err.Error())
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
