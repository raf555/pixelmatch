package testutil

import (
	"image"
	"math/rand/v2"
)

// MakeRGBA fills a width*height*4 buffer with a per-pixel function.
func MakeRGBA(w, h int, fn func(x, y int) (r, g, b, a uint8)) []byte {
	buf := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			r, g, b, a := fn(x, y)
			i := (y*w + x) * 4
			buf[i+0] = r
			buf[i+1] = g
			buf[i+2] = b
			buf[i+3] = a
		}
	}
	return buf
}

func BytesToNRGBA(pix []byte, w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, pix)
	return img
}

func BenchData(w, h int) (img1, img2, out []byte) {
	n := w * h * 4
	img1 = make([]byte, n)
	img2 = make([]byte, n)
	out = make([]byte, n)
	r := rand.New(rand.NewPCG(1, 2))
	for i := range n {
		img1[i] = byte(r.IntN(256))
	}
	copy(img2, img1)
	for i := 0; i < w*h/20; i++ {
		idx := r.IntN(w*h) * 4
		img2[idx] ^= 0xff
		img2[idx+1] ^= 0x77
	}
	return
}
