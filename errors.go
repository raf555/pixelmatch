package pixelmatch

import v7 "github.com/raf555/pixelmatch/v7"

// These hold the same values as their v7 counterparts, so errors.Is
// matches across both packages.
var (
	// ErrInvalidDimensions is [v7.ErrInvalidDimensions].
	ErrInvalidDimensions = v7.ErrInvalidDimensions

	// ErrNilImage is [v7.ErrNilImage].
	ErrNilImage = v7.ErrNilImage

	// ErrDimensionMismatch is [v7.ErrDimensionMismatch].
	ErrDimensionMismatch = v7.ErrDimensionMismatch

	// ErrOutputDimensionMismatch is [v7.ErrOutputDimensionMismatch].
	ErrOutputDimensionMismatch = v7.ErrOutputDimensionMismatch
)
