package pixelmatch

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/raf555/pixelmatch/v8/internal/testutil"
)

// BenchmarkFixtures mirrors upstream's bench.js: it compares fixture pairs
// 1 to 7 with default options and no diff output. These are rendered,
// mostly opaque images, unlike the random noise in BenchData.
func BenchmarkFixtures(b *testing.B) {
	type pair struct {
		a, b []byte
		w, h int
	}
	load := func(name string) ([]byte, int, int) {
		f, err := os.Open("../testutil/testdata/" + name + ".png")
		if err != nil {
			b.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			b.Fatal(err)
		}
		r := img.Bounds()
		dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
		return dst.Pix, r.Dx(), r.Dy()
	}

	var pairs []pair
	var total int64
	for i := 1; i <= 7; i++ {
		a, w, h := load(fmt.Sprintf("%da", i))
		bb, _, _ := load(fmt.Sprintf("%db", i))
		pairs = append(pairs, pair{a, bb, w, h})
		total += int64(len(a) * 2)
	}
	opts := DefaultOptions()

	b.SetBytes(total)
	for b.Loop() {
		for _, p := range pairs {
			_, _ = Match(p.a, p.b, nil, p.w, p.h, &opts)
		}
	}
}

// Low-level byte API benchmarks.

func BenchmarkMatch800x600(b *testing.B) {
	img1, img2, out := testutil.BenchData(800, 600)
	opts := DefaultOptions()

	b.SetBytes(int64(len(img1) * 2))
	for b.Loop() {
		_, _ = Match(img1, img2, out, 800, 600, &opts)
	}
}

func BenchmarkMatchIdentical800x600(b *testing.B) {
	img1, _, out := testutil.BenchData(800, 600)
	img2 := make([]byte, len(img1))
	copy(img2, img1)
	opts := DefaultOptions()

	b.SetBytes(int64(len(img1) * 2))
	for b.Loop() {
		_, _ = Match(img1, img2, out, 800, 600, &opts)
	}
}

func BenchmarkMatchNoOutput(b *testing.B) {
	img1, img2, _ := testutil.BenchData(800, 600)
	opts := DefaultOptions()

	b.SetBytes(int64(len(img1) * 2))
	for b.Loop() {
		_, _ = Match(img1, img2, nil, 800, 600, &opts)
	}
}
