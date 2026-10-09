package turzx

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// FrameSender accepts the renderer's logical RGBA frame.
type FrameSender interface {
	SendFrame(*image.RGBA) error
	Exit() error
	Close() error
}

func OpenFrameSender(id string) (FrameSender, error) { return openFrameSender(id) }

type jpegFrameSender struct{ conn *Conn }

func (s *jpegFrameSender) SendFrame(img *image.RGBA) error {
	if img == nil || img.Bounds().Dx() != 1920 || img.Bounds().Dy() != 462 {
		return fmt.Errorf("TURZX 9.2-inch frame must be 1920x462")
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rotateClockwise(img), &jpeg.Options{Quality: 85}); err != nil {
		return err
	}
	return s.conn.SendJPEG(buf.Bytes())
}
func (s *jpegFrameSender) Exit() error  { return s.conn.Restart() }
func (s *jpegFrameSender) Close() error { return s.conn.Close() }

func rotateClockwise(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			srcOffset := src.PixOffset(b.Min.X+x, b.Min.Y+y)
			dstOffset := dst.PixOffset(b.Dy()-1-y, x)
			copy(dst.Pix[dstOffset:dstOffset+4], src.Pix[srcOffset:srcOffset+4])
		}
	}
	return dst
}
