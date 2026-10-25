package pixelmatch_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/raf555/pixelmatch/v7"
)

// whiteImage returns a 4×4 opaque white image.
func whiteImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return img
}

var black = color.NRGBA{A: 255}

func ExampleCompare() {
	a := whiteImage()
	b := whiteImage()
	b.SetNRGBA(1, 1, black)

	n, err := pixelmatch.Compare(a, b)
	if err != nil {
		panic(err)
	}
	fmt.Println("mismatched pixels:", n)
	// Output: mismatched pixels: 1
}

func ExampleCompareToImage() {
	a := whiteImage()
	b := whiteImage()
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
