package turzx

import (
	"encoding/binary"
	"fmt"
	"image"
)

const revAWidth, revAHeight = 480, 320

// revAStripWidth is the width of each vertical strip; it must divide revAWidth.
const revAStripWidth = 8

// revAResetCommand restarts the Rev.A controller, matching the 9.2-inch exit behavior.
func revAResetCommand() []byte { return []byte{0, 0, 0, 0, 0, 101} }

func revAOrientation() []byte {
	cmd := make([]byte, 16)
	cmd[5] = 121
	cmd[6] = 102 // landscape orientation (2) with the protocol's +100 encoding
	cmd[7], cmd[8] = byte(revAWidth>>8), byte(revAWidth&0xff)
	cmd[9], cmd[10] = byte(revAHeight>>8), byte(revAHeight&0xff)
	return cmd
}

// revABitmapCommand packs inclusive start/end coordinates into the controller's 6-byte window header.
func revABitmapCommand(x, y, ex, ey int) []byte {
	return []byte{byte(x >> 2), byte(((x & 3) << 6) + (y >> 4)), byte(((y & 15) << 4) + (ex >> 6)), byte(((ex & 63) << 2) + (ey >> 8)), byte(ey), 197}
}

func revACheckFrame(img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("TURZX Rev.A frame is nil")
	}
	if img.Bounds().Dx() != revAWidth || img.Bounds().Dy() != revAHeight {
		return fmt.Errorf("TURZX Rev.A frame must be %dx%d, got %dx%d", revAWidth, revAHeight, img.Bounds().Dx(), img.Bounds().Dy())
	}
	return nil
}

// revARGB565LERect converts r (frame-relative) in row-major order within r.
func revARGB565LERect(img *image.RGBA, r image.Rectangle) ([]byte, error) {
	if err := revACheckFrame(img); err != nil {
		return nil, err
	}
	if !r.In(image.Rect(0, 0, revAWidth, revAHeight)) || r.Empty() {
		return nil, fmt.Errorf("TURZX Rev.A rectangle %v is outside the frame", r)
	}
	out := make([]byte, r.Dx()*r.Dy()*2)
	b := img.Bounds()
	o := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			pixel := uint16(img.Pix[i]>>3)<<11 | uint16(img.Pix[i+1]>>2)<<5 | uint16(img.Pix[i+2]>>3)
			binary.LittleEndian.PutUint16(out[o:], pixel)
			o += 2
		}
	}
	return out, nil
}

// revARects splits the frame into revAStripWidth-wide full-height strips, left to right.
func revARects() []image.Rectangle {
	rects := make([]image.Rectangle, 0, revAWidth/revAStripWidth)
	for x := 0; x < revAWidth; x += revAStripWidth {
		rects = append(rects, image.Rect(x, 0, x+revAStripWidth, revAHeight))
	}
	return rects
}

func revAChunks(pixels []byte) [][]byte {
	const chunkSize = revAWidth * 8
	chunks := make([][]byte, 0, (len(pixels)+chunkSize-1)/chunkSize)
	for len(pixels) > 0 {
		n := min(len(pixels), chunkSize)
		chunks = append(chunks, pixels[:n])
		pixels = pixels[n:]
	}
	return chunks
}
