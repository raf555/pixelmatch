package pixelmatch

import "testing"

func gray(v uint8) []byte { return []byte{v, v, v, 255} }

func matchN(t *testing.T, img1, img2 []byte, w, h int, mutate func(*Options)) int {
	t.Helper()
	o := DefaultOptions()
	if mutate != nil {
		mutate(&o)
	}
	n, err := Match(img1, img2, nil, w, h, &o)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestToeCorrectedLightness pins the behavior of the Lr toe correction on
// near-black differences, which raw OKLab L would over-expand.
func TestToeCorrectedLightness(t *testing.T) {
	if got := matchN(t, gray(0), gray(13), 1, 1, nil); got != 0 {
		t.Errorf("gray 0 vs 13: got %d diffs, want 0", got)
	}
	if got := matchN(t, gray(0), gray(23), 1, 1, nil); got != 1 {
		t.Errorf("gray 0 vs 23: got %d diffs, want 1", got)
	}
}

// TestSaturatedHuePairs covers mapbox/pixelmatch#127: the YIQ metric scored
// these pairs near-identically, while OKLab HyAB separates them correctly at
// the default threshold.
func TestSaturatedHuePairs(t *testing.T) {
	tests := []struct {
		name                   string
		r1, g1, b1, r2, g2, b2 uint8
		want                   int
	}{
		{"very different", 41, 56, 157, 41, 56, 0, 1},
		{"clearly different", 39, 44, 92, 39, 44, 14, 1},
		{"nearly identical", 0, 254, 252, 94, 254, 252, 0},
		{"imperceptible", 0, 254, 252, 47, 254, 252, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img1 := []byte{tc.r1, tc.g1, tc.b1, 255}
			img2 := []byte{tc.r2, tc.g2, tc.b2, 255}
			if got := matchN(t, img1, img2, 1, 1, nil); got != tc.want {
				t.Errorf("got %d diffs, want %d", got, tc.want)
			}
		})
	}
}

// TestWindowSize checks that windowed mode reports the densest local diff
// count rather than the image-wide total.
func TestWindowSize(t *testing.T) {
	const w, h = 10, 10
	img1 := make([]byte, w*h*4)
	img2 := make([]byte, w*h*4)
	for i := range img1 {
		img1[i] = 255
		img2[i] = 255
	}
	// A dense 3x3 block of black diff pixels at (2,2)-(4,4).
	for y := 2; y < 5; y++ {
		for x := 2; x < 5; x++ {
			p := (y*w + x) * 4
			img2[p], img2[p+1], img2[p+2] = 0, 0, 0
		}
	}

	withWindow := func(n int) func(*Options) {
		return func(o *Options) {
			o.IncludeAA = true
			o.WindowSize = n
		}
	}

	tests := []struct {
		name       string
		windowSize int
		want       int
	}{
		{"unset reports the total", 0, 9},
		{"negative reports the total", -1, 9},
		{"3x3 captures the whole block", 3, 9},
		{"2x2 captures at most four", 2, 4},
		{"1x1 captures a single pixel", 1, 1},
		{"larger than the image is clamped", 100, 9},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchN(t, img1, img2, w, h, withWindow(tc.windowSize)); got != tc.want {
				t.Errorf("windowSize %d: got %d, want %d", tc.windowSize, got, tc.want)
			}
		})
	}

	t.Run("identical images report zero", func(t *testing.T) {
		if got := matchN(t, img1, img1, w, h, withWindow(3)); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}

// TestWindowSizeExcludesAntialiasing verifies that pixels dismissed as
// anti-aliasing never contribute to a window count, even though they still
// occupy a slot in the internal mask.
func TestWindowSizeExcludesAntialiasing(t *testing.T) {
	const w, h = 16, 16
	img1 := make([]byte, w*h*4)
	img2 := make([]byte, w*h*4)
	// A one-pixel-wide diagonal line, which the AA detector rejects, against
	// an otherwise flat background.
	for y := range h {
		for x := range w {
			p := (y*w + x) * 4
			img1[p], img1[p+1], img1[p+2], img1[p+3] = 255, 255, 255, 255
			img2[p], img2[p+1], img2[p+2], img2[p+3] = 255, 255, 255, 255
			if x == y {
				img2[p], img2[p+1], img2[p+2] = 0, 0, 0
			}
		}
	}

	total := matchN(t, img1, img2, w, h, nil)
	windowed := matchN(t, img1, img2, w, h, func(o *Options) { o.WindowSize = 4 })
	if windowed > total {
		t.Errorf("windowed count %d exceeds total %d", windowed, total)
	}

	// The whole-image window is the degenerate case and must equal the total.
	if got := matchN(t, img1, img2, w, h, func(o *Options) { o.WindowSize = w }); got != total {
		t.Errorf("full-size window: got %d, want %d", got, total)
	}
}
