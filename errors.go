package pixelmatch

import (
	"errors"

	"github.com/raf555/pixelmatch/internal/pixelmatch"
)

var (
	// ErrInvalidDimensions is returned when width or height is not a positive integer.
	ErrInvalidDimensions = pixelmatch.ErrInvalidDimensions

	// ErrNilImage is returned when a required image argument is nil.
	ErrNilImage = errors.New("pixelmatch: image must not be nil")

	// ErrDimensionMismatch is returned when img1 and img2 have different bounds.
	// The returned error appends the actual dimensions for context.
	ErrDimensionMismatch = errors.New("pixelmatch: image dimensions do not match")

	// ErrOutputDimensionMismatch is returned when the WithOutput target size does
	// not match the input images. The returned error appends the actual dimensions
	// for context.
	ErrOutputDimensionMismatch = errors.New("pixelmatch: output dimensions do not match")
)
