// Package pixelmatch forwards to [github.com/raf555/pixelmatch/v7], which
// holds the same mapbox/pixelmatch 7.2.0 port under a module path that
// matches the upstream major version.
//
// Deprecated: Use github.com/raf555/pixelmatch/v7, or
// github.com/raf555/pixelmatch/v8 for upstream 8.x.
package pixelmatch

import (
	"image"

	v7 "github.com/raf555/pixelmatch/v7"
)

// Compare calls [v7.Compare].
func Compare(img1, img2 image.Image, opts ...Option) (int, error) {
	return v7.Compare(img1, img2, opts...)
}

// CompareToImage calls [v7.CompareToImage].
func CompareToImage(img1, img2 image.Image, opts ...Option) (*image.NRGBA, int, error) {
	return v7.CompareToImage(img1, img2, opts...)
}
