package screenshot

import (
	"bytes"
	_ "image/png"

	"github.com/disintegration/imaging"
	"github.com/skrashevich/go-webp"
)

const thumbWidth = 640

func resizeToWebP(pngData []byte) ([]byte, error) {
	img, err := decodePNG(pngData)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w > thumbWidth {
		h = h * thumbWidth / w
		w = thumbWidth
		img = imaging.Resize(img, w, h, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossy: true, Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
