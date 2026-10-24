package pixelmatch_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/raf555/pixelmatch/v8"
)

// whiteImage returns a w×h opaque white image.
func whiteImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return img
}

var black = color.NRGBA{A: 255}

func ExampleCompare() {
	a := whiteImage(4, 4)
	b := whiteImage(4, 4)
	b.SetNRGBA(1, 1, black)

	n, err := pixelmatch.Compare(a, b)
	if err != nil {
		panic(err)
	}
	fmt.Println("mismatched pixels:", n)
	// Output: mismatched pixels: 1
}

func ExampleCompareToImage() {
	a := whiteImage(4, 4)
	b := whiteImage(4, 4)
	b.SetNRGBA(1, 1, black)

	diff, n, err := pixelmatch.CompareToImage(a, b, pixelmatch.WithDiffColor(255, 0, 255))
	if err != nil {
		panic(err)
	}
	fmt.Println("mismatched pixels:", n)
	fmt.Println("changed pixel:", diff.NRGBAAt(1, 1))
	// Output:
	// mismatched pixels: 1
	// changed pixel: {255 0 255 255}
}

// A test can fail on dense clusters of change and tolerate scattered noise.
// Here b has 16 isolated noise pixels and one 4×4 block of real change.
func ExampleWithWindowSize() {
	a := whiteImage(64, 64)
	b := whiteImage(64, 64)

	// Scattered noise, spaced so no 8×8 window holds more than one pixel.
	for y := 2; y < 30; y += 9 {
		for x := 2; x < 30; x += 9 {
			b.SetNRGBA(x, y, black)
		}
	}
	// A dense 4×4 block.
	for y := 48; y < 52; y++ {
		for x := 48; x < 52; x++ {
			b.SetNRGBA(x, y, black)
		}
	}

	total, _ := pixelmatch.Compare(a, b)

	const window = 8
	densest, _ := pixelmatch.Compare(a, b, pixelmatch.WithWindowSize(window))
	density := float64(densest) / (window * window)

	fmt.Println("total:", total)
	fmt.Println("densest 8×8 window:", densest)
	fmt.Println("regression:", density > 0.1)
	// Output:
	// total: 32
	// densest 8×8 window: 16
	// regression: true
}
