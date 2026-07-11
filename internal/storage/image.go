package storage

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

const maxImageWidth = 1600

// ProcessImage decodes an image, resizes it if wider than maxImageWidth, and
// encodes it as WebP.
func ProcessImage(r io.Reader) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() > maxImageWidth {
		img = imaging.Resize(img, maxImageWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("encoding webp: %w", err)
	}
	return buf.Bytes(), nil
}
