package pixelmatch

import (
	"testing"

	"github.com/raf555/pixelmatch/internal/testutil"
)

func TestIdenticalImages(t *testing.T) {
	w, h := 32, 32
	img := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return uint8(x * 8), uint8(y * 8), 128, 255
	})
	img2 := make([]byte, len(img))
	copy(img2, img)

	out := make([]byte, len(img))
	n, err := Match(img, img2, out, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("identical images: got %d diff pixels, want 0", n)
	}
	// Output should contain a faded grayscale of img (not all zeros).
	allZero := true
	for _, b := range out {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("identical images: output should be faded grayscale, got all zeros")
	}
}

func TestIdenticalNoOutput(t *testing.T) {
	w, h := 8, 8
	img := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 100, 100, 100, 255
	})
	img2 := make([]byte, len(img))
	copy(img2, img)
	n, err := Match(img, img2, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}

func TestCompletelyDifferent(t *testing.T) {
	w, h := 16, 16
	// All-black vs all-white — every pixel should differ.
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 0, 0, 0, 255
	})
	img2 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 255, 255, 255, 255
	})
	out := make([]byte, len(img1))
	n, err := Match(img1, img2, out, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("got %d diff pixels, want %d", n, w*h)
	}
	// First pixel should be drawn red (default diffColor) with full alpha.
	if out[0] != 255 || out[1] != 0 || out[2] != 0 || out[3] != 255 {
		t.Errorf("first diff pixel = [%d %d %d %d], want [255 0 0 255]",
			out[0], out[1], out[2], out[3])
	}
}

func TestPartialDifference(t *testing.T) {
	w, h := 10, 10
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 50, 50, 50, 255
	})
	img2 := make([]byte, len(img1))
	copy(img2, img1)
	// Change exactly 7 pixels.
	for i := range 7 {
		idx := i * 4
		img2[idx+0] = 200
		img2[idx+1] = 200
		img2[idx+2] = 200
	}
	n, err := Match(img1, img2, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("got %d, want 7", n)
	}
}

func TestThresholdSensitivity(t *testing.T) {
	w, h := 8, 8
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 100, 100, 100, 255
	})
	// Tiny per-pixel change.
	img2 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 105, 105, 105, 255
	})

	// Loose threshold: should detect nothing.
	loose := DefaultOptions()
	loose.Threshold = 0.5
	n, err := Match(img1, img2, nil, w, h, &loose)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("loose threshold: got %d, want 0", n)
	}

	// Tight threshold: should detect everything.
	tight := DefaultOptions()
	tight.Threshold = 0.0
	n, err = Match(img1, img2, nil, w, h, &tight)
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("tight threshold: got %d, want %d", n, w*h)
	}
}

func TestDiffColorAlt(t *testing.T) {
	w, h := 4, 4
	// img1 light, img2 dark in one half; img1 dark, img2 light in the other.
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		if x < 2 {
			return 240, 240, 240, 255 // light
		}
		return 20, 20, 20, 255 // dark
	})
	img2 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		if x < 2 {
			return 20, 20, 20, 255 // dark (img2 darker)
		}
		return 240, 240, 240, 255 // light (img2 lighter)
	})

	opts := DefaultOptions()
	opts.DiffColor = [3]uint8{255, 0, 0}    // red for added/lighter-in-img2
	opts.DiffColorAlt = [3]uint8{0, 0, 255} // blue for removed/darker-in-img2
	opts.HasDiffColorAlt = true

	out := make([]byte, len(img1))
	n, err := Match(img1, img2, out, w, h, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if n != w*h {
		t.Errorf("got %d diff pixels, want %d", n, w*h)
	}

	// Pixel at (0,0): img2 is darker → delta<0 → DiffColorAlt (blue).
	if out[0] != 0 || out[1] != 0 || out[2] != 255 {
		t.Errorf("dark-in-img2 pixel: got [%d %d %d], want blue", out[0], out[1], out[2])
	}
	// Pixel at (2,0): img2 is lighter → delta>0 → DiffColor (red).
	i := (0*w + 2) * 4
	if out[i] != 255 || out[i+1] != 0 || out[i+2] != 0 {
		t.Errorf("light-in-img2 pixel: got [%d %d %d], want red", out[i], out[i+1], out[i+2])
	}
}

func TestDiffMask(t *testing.T) {
	w, h := 4, 4
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		return 100, 100, 100, 255
	})
	img2 := make([]byte, len(img1))
	copy(img2, img1)
	// Change one pixel.
	img2[0], img2[1], img2[2] = 200, 50, 50

	opts := DefaultOptions()
	opts.DiffMask = true
	out := make([]byte, len(img1))
	n, err := Match(img1, img2, out, w, h, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d, want 1", n)
	}
	// First pixel: red (diff). All other pixels in the output must remain
	// untouched (zero) because diffMask suppresses background drawing.
	if out[0] != 255 || out[1] != 0 || out[2] != 0 || out[3] != 255 {
		t.Errorf("diff pixel: got [%d %d %d %d]", out[0], out[1], out[2], out[3])
	}
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Errorf("diffMask should leave untouched pixels as 0, got out[%d]=%d", i, out[i])
			break
		}
	}
}

func TestSizeMismatchErrors(t *testing.T) {
	img1 := make([]byte, 16) // 2x2 RGBA = 16 bytes
	img2 := make([]byte, 20)
	if _, err := Match(img1, img2, nil, 2, 2, nil); err == nil {
		t.Error("expected error on mismatched sizes")
	}
	if _, err := Match(img1, img1, nil, 3, 3, nil); err == nil {
		t.Error("expected error on width/height mismatch")
	}
}

// TestAntialiasingDetection builds a synthetic AA-like pattern. The center
// pixel of a small gradient has both brighter and darker neighbors, with
// several of those neighbors sharing colors among themselves. The
// implementation should classify it as AA and not count it as a real diff.
func TestAntialiasingDetection(t *testing.T) {
	w, h := 5, 5
	// img1: a vertical edge with 1-px gradient.
	//   columns 0,1 = black ; column 2 = mid-gray ; columns 3,4 = white
	img1 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		switch {
		case x < 2:
			return 0, 0, 0, 255
		case x == 2:
			return 128, 128, 128, 255
		default:
			return 255, 255, 255, 255
		}
	})
	// img2: same edge but shifted by one pixel — typical AA difference.
	img2 := testutil.MakeRGBA(w, h, func(x, y int) (uint8, uint8, uint8, uint8) {
		switch {
		case x < 1:
			return 0, 0, 0, 255
		case x == 1:
			return 128, 128, 128, 255
		default:
			return 255, 255, 255, 255
		}
	})

	// With AA detection on (default), the AA pixels should be ignored or
	// counted minimally; with includeAA on, more pixels should differ.
	nAA, err := Match(img1, img2, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.IncludeAA = true
	nNoAA, err := Match(img1, img2, nil, w, h, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !(nAA < nNoAA) {
		t.Errorf("expected AA-aware count (%d) < AA-included count (%d)", nAA, nNoAA)
	}
}

// TestAlphaBlending verifies semi-transparent inputs are handled correctly:
// a fully-transparent pixel should equal any other fully-transparent pixel.
func TestAlphaBlending(t *testing.T) {
	w, h := 2, 1
	// img1: two fully-transparent pixels with different RGB values.
	img1 := []byte{255, 0, 0, 0, 0, 255, 0, 0}
	// img2: two fully-transparent pixels with yet other RGB values.
	img2 := []byte{0, 0, 255, 0, 128, 128, 128, 0}
	n, err := Match(img1, img2, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Transparent pixels blend to white in both images, so should match.
	if n != 0 {
		t.Errorf("fully-transparent pixels should match, got %d diffs", n)
	}
}
