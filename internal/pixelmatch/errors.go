package pixelmatch

import "errors"

var (
	// ErrInvalidDimensions is returned when width or height is not a positive integer.
	ErrInvalidDimensions = errors.New("pixelmatch: width and height must be positive")

	// ErrDataSizeMismatch is returned when the length of img1 or img2 does not
	// match the expected size for the given dimensions.
	ErrDataSizeMismatch = errors.New("pixelmatch: image data size does not match width/height")

	// ErrOutputSizeMismatch is returned when the output buffer length does not
	// match the expected size for the given dimensions.
	ErrOutputSizeMismatch = errors.New("pixelmatch: output size does not match width/height")
)
