package pixelmatch

import (
	"testing"

	"github.com/raf555/pixelmatch/internal/testutil"
)

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
