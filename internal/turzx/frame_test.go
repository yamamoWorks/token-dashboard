package turzx

import (
	"image"
	"image/color"
	"testing"
)

func TestRotateClockwise(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	top := color.RGBA{255, 0, 0, 255}
	src.SetRGBA(0, 0, top) // top-left goes to top-right
	dst := rotateClockwise(src)
	if dst.Bounds().Dx() != 2 || dst.Bounds().Dy() != 3 || dst.RGBAAt(1, 0) != top {
		t.Fatalf("size %v, top-right %v", dst.Bounds(), dst.RGBAAt(1, 0))
	}
}
