package internal

import (
	"bytes"
	"crypto/sha256"

	"github.com/coregx/gxpdf/creator"
)

// ImageSet decodes image bytes once per distinct content and shares the result
// between layout (intrinsic size) and rendering, so that one image placed many
// times, such as a logo in every header, is decoded once and written as a
// single image XObject. Like the Builder, it is not safe for concurrent use.
type ImageSet struct {
	byDigest map[[sha256.Size]byte]decodedImage
}

// decodedImage is the outcome of decoding one image content; failures are
// kept so that the same bad bytes are not decoded again.
type decodedImage struct {
	img *creator.Image
	err error
}

// NewImageSet returns an empty ImageSet.
func NewImageSet() *ImageSet {
	return &ImageSet{byDigest: make(map[[sha256.Size]byte]decodedImage)}
}

// Load returns the decoded image for data (JPEG or PNG), decoding it on the
// first call for that content.
func (s *ImageSet) Load(data []byte) (*creator.Image, error) {
	key := sha256.Sum256(data)
	if d, ok := s.byDigest[key]; ok {
		return d.img, d.err
	}
	img, err := creator.LoadImageFromReader(bytes.NewReader(data))
	s.byDigest[key] = decodedImage{img: img, err: err}
	return img, err
}
