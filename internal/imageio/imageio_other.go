//go:build !darwin

package imageio

import (
	"bytes"
	"errors"
	"image"

	"github.com/gen2brain/h265/heic"
	xdraw "golang.org/x/image/draw"
)

// Decode decodes a HEIC picture into sRGB pixels, scaled down to at most
// maxWidth pixels wide when maxWidth > 0. Other formats ui reads itself.
//
// The decoder is pure Go, so it builds for phones as it does for the other
// desktops, where ImageIO has no HEIC to offer.
func Decode(data []byte, maxWidth int) (*image.RGBA, error) {
	if len(data) == 0 {
		return nil, errors.New("imageio: no data")
	}
	img, err := heic.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return nil, errors.New("imageio: empty image")
	}
	if maxWidth > 0 && w > maxWidth {
		h = max(1, h*maxWidth/w)
		w = maxWidth
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return out, nil
}
