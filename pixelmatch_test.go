package pixelmatch

import (
	"bytes"
	"errors"
	"image"
	"testing"

	v7 "github.com/raf555/pixelmatch/v7"
)

// testImages returns two 32×32 images with gradients, a semi-transparent
// band and a shifted edge, so every option changes the result.
func testImages() (a, b *image.NRGBA) {
	const size = 32
	a = image.NewNRGBA(image.Rect(0, 0, size, size))
	b = image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			i := a.PixOffset(x, y)
			alpha := uint8(255)
			if y >= 24 {
				alpha = uint8(64 + x*4)
			}
			copy(a.Pix[i:], []byte{uint8(x * 8), uint8(y * 8), 128, alpha})
			copy(b.Pix[i:], a.Pix[i:i+4])
			if x > 16 {
				copy(b.Pix[i:], []byte{20, 20, 20, alpha})
			}
			if x == y {
				copy(b.Pix[i:], []byte{255, 255, 255, 255})
			}
		}
	}
	return a, b
}

func TestForwardsToV7(t *testing.T) {
	a, b := testImages()

	tests := []struct {
		name string
		v1   []Option
		v7   []v7.Option
	}{
		{"defaults", nil, nil},
		{"threshold", []Option{WithThreshold(0.01)}, []v7.Option{v7.WithThreshold(0.01)}},
		{"include AA", []Option{WithIncludeAA(true)}, []v7.Option{v7.WithIncludeAA(true)}},
		{"alpha", []Option{WithAlpha(0.7)}, []v7.Option{v7.WithAlpha(0.7)}},
		{"AA color", []Option{WithAAColor(0, 192, 0)}, []v7.Option{v7.WithAAColor(0, 192, 0)}},
		{"diff color", []Option{WithDiffColor(255, 0, 255)}, []v7.Option{v7.WithDiffColor(255, 0, 255)}},
		{"diff color alt", []Option{WithDiffColorAlt(0, 255, 0)}, []v7.Option{v7.WithDiffColorAlt(0, 255, 0)}},
		{"diff mask", []Option{WithDiffMask(true)}, []v7.Option{v7.WithDiffMask(true)}},
		{"no checkerboard", []Option{WithCheckerboard(false)}, []v7.Option{v7.WithCheckerboard(false)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantDiff, wantN, err := v7.CompareToImage(a, b, tt.v7...)
			if err != nil {
				t.Fatal(err)
			}

			gotDiff, gotN, err := CompareToImage(a, b, tt.v1...)
			if err != nil {
				t.Fatal(err)
			}
			if gotN != wantN {
				t.Errorf("CompareToImage: n = %d, want %d", gotN, wantN)
			}
			if !bytes.Equal(gotDiff.Pix, wantDiff.Pix) {
				t.Error("CompareToImage: diff image differs from v7")
			}

			out := image.NewNRGBA(a.Bounds())
			n, err := Compare(a, b, append(tt.v1, WithOutput(out))...)
			if err != nil {
				t.Fatal(err)
			}
			if n != wantN {
				t.Errorf("Compare: n = %d, want %d", n, wantN)
			}
			if !bytes.Equal(out.Pix, wantDiff.Pix) {
				t.Error("Compare with WithOutput: diff image differs from v7")
			}
		})
	}
}

func TestOptionsMixWithV7(t *testing.T) {
	a, b := testImages()
	want, err := v7.Compare(a, b, v7.WithThreshold(0.01), v7.WithIncludeAA(true))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Compare(a, b, WithThreshold(0.01), v7.WithIncludeAA(true))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("n = %d, want %d", got, want)
	}
}

func TestErrorsMatchV7(t *testing.T) {
	small := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	large := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	empty := image.NewNRGBA(image.Rect(0, 0, 0, 0))

	tests := []struct {
		name  string
		run   func() error
		v1Err error
		v7Err error
	}{
		{
			name:  "nil image",
			run:   func() error { _, err := Compare(nil, small); return err },
			v1Err: ErrNilImage, v7Err: v7.ErrNilImage,
		},
		{
			name:  "dimension mismatch",
			run:   func() error { _, err := Compare(small, large); return err },
			v1Err: ErrDimensionMismatch, v7Err: v7.ErrDimensionMismatch,
		},
		{
			name: "output dimension mismatch",
			run: func() error {
				_, err := Compare(small, small, WithOutput(large))
				return err
			},
			v1Err: ErrOutputDimensionMismatch, v7Err: v7.ErrOutputDimensionMismatch,
		},
		{
			name:  "invalid dimensions",
			run:   func() error { _, err := Compare(empty, empty); return err },
			v1Err: ErrInvalidDimensions, v7Err: v7.ErrInvalidDimensions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if !errors.Is(err, tt.v1Err) {
				t.Errorf("got err %v, want %v", err, tt.v1Err)
			}
			if !errors.Is(err, tt.v7Err) {
				t.Errorf("got err %v, want v7's %v", err, tt.v7Err)
			}
		})
	}
}
