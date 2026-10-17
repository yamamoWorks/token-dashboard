package compact

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
)

const (
	sourceWidth  = 1920
	sourceHeight = 462
)

// frameFromImage accepts only a full upstream Canvas PNG or the upstream
// portrait JPEG used by display.black(). A JPEG is never used for picture
// pixels: only a verified black JPEG may request a solid black Rev.A frame.
func frameFromImage(data []byte) (*image.RGBA, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("compact image header: %w", err)
	}
	switch format {
	case "png":
		if config.Width != sourceWidth || config.Height != sourceHeight {
			return nil, fmt.Errorf("compact PNG must be %dx%d, got %dx%d", sourceWidth, sourceHeight, config.Width, config.Height)
		}
		src, decodedFormat, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode compact PNG: %w", err)
		}
		if decodedFormat != "png" || src.Bounds().Dx() != sourceWidth || src.Bounds().Dy() != sourceHeight {
			return nil, fmt.Errorf("unexpected decoded compact PNG: %s %v", decodedFormat, src.Bounds())
		}
		dst := image.NewRGBA(image.Rect(0, 0, Width, Height))
		// Copy pixels to an independent 480x320 frame, not a view over the
		// 1920x462 image. The driver reads this as RGB565LE with no JPEG step.
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
		return dst, nil
	case "jpeg":
		// The sole JPEG produced by display.Run is the black 462x1920
		// portrait image used while the user is away. Do not interpret
		// arbitrary JPEGs as content or silently turn them black.
		if config.Width != sourceHeight || config.Height != sourceWidth {
			return nil, fmt.Errorf("compact black JPEG must be %dx%d, got %dx%d", sourceHeight, sourceWidth, config.Width, config.Height)
		}
		src, decodedFormat, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode compact black JPEG: %w", err)
		}
		if decodedFormat != "jpeg" {
			return nil, fmt.Errorf("unexpected decoded compact black image format %q", decodedFormat)
		}
		bounds := src.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := src.At(x, y).RGBA()
				if r != 0 || g != 0 || b != 0 {
					return nil, fmt.Errorf("compact black JPEG contains non-black pixel at (%d,%d)", x, y)
				}
			}
		}
		dst := image.NewRGBA(image.Rect(0, 0, Width, Height))
		for i := 3; i < len(dst.Pix); i += 4 {
			dst.Pix[i] = 255
		}
		return dst, nil
	default:
		return nil, fmt.Errorf("unsupported compact image format %q", format)
	}
}
