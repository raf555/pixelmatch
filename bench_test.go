package pixelmatch

import (
	"image"
	"testing"

	"github.com/raf555/pixelmatch/internal/testutil"
)

// High-level image.Image API benchmarks. NRGBA with standard layout
// should go through the zero-copy fast path and match the byte API.

func BenchmarkCompareNRGBA800x600(b *testing.B) {
	pix1, pix2, _ := testutil.BenchData(800, 600)
	img1 := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	img2 := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	copy(img1.Pix, pix1)
	copy(img2.Pix, pix2)
	out := image.NewNRGBA(image.Rect(0, 0, 800, 600))

	b.SetBytes(int64(len(pix1) * 2))
	for b.Loop() {
		_, _ = Compare(img1, img2, WithOutput(out))
	}
}

func BenchmarkCompareNoOutputNRGBA(b *testing.B) {
	pix1, pix2, _ := testutil.BenchData(800, 600)
	img1 := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	img2 := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	copy(img1.Pix, pix1)
	copy(img2.Pix, pix2)

	b.SetBytes(int64(len(pix1) * 2))
	for b.Loop() {
		_, _ = Compare(img1, img2)
	}
}

// RGBA (premultiplied) needs un-premultiplication on input, so this
// should be measurably slower than the NRGBA path.
func BenchmarkCompareRGBA800x600(b *testing.B) {
	pix1, pix2, _ := testutil.BenchData(800, 600)
	img1 := image.NewRGBA(image.Rect(0, 0, 800, 600))
	img2 := image.NewRGBA(image.Rect(0, 0, 800, 600))
	copy(img1.Pix, pix1)
	copy(img2.Pix, pix2)
	for i := 3; i < len(img1.Pix); i += 4 {
		img1.Pix[i] = 255
		img2.Pix[i] = 255
	}
	out := image.NewNRGBA(image.Rect(0, 0, 800, 600))

	b.SetBytes(int64(len(pix1) * 2))
	for b.Loop() {
		_, _ = Compare(img1, img2, WithOutput(out))
	}
}
