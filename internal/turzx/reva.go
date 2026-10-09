package turzx

import (
	"encoding/binary"
	"fmt"
	"image"
)

const revAWidth, revAHeight = 480, 320

func revAOrientation() []byte {
	cmd := make([]byte, 16)
	cmd[5] = 121
	cmd[6] = 102 // landscape orientation (2) with the protocol's +100 encoding
	cmd[7], cmd[8] = byte(revAWidth>>8), byte(revAWidth&0xff)
	cmd[9], cmd[10] = byte(revAHeight>>8), byte(revAHeight&0xff)
	return cmd
}

func revABitmapCommand(width, height int) []byte {
	// Pack inclusive start/end coordinates into the controller's 6-byte window header.
	x, y, ex, ey := 0, 0, width-1, height-1
	return []byte{byte(x >> 2), byte(((x & 3) << 6) + (y >> 4)), byte(((y & 15) << 4) + (ex >> 6)), byte(((ex & 63) << 2) + (ey >> 8)), byte(ey), 197}
}

func revARGB565LE(img *image.RGBA) ([]byte, error) {
	if img == nil || img.Bounds().Dx() != revAWidth || img.Bounds().Dy() != revAHeight {
		if img == nil {
			return nil, fmt.Errorf("TURZX Rev.A frame is nil")
		}
		return nil, fmt.Errorf("TURZX Rev.A frame must be %dx%d, got %dx%d", revAWidth, revAHeight, img.Bounds().Dx(), img.Bounds().Dy())
	}
	out := make([]byte, revAWidth*revAHeight*2)
	b := img.Bounds()
	for y := 0; y < revAHeight; y++ {
		for x := 0; x < revAWidth; x++ {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			pixel := uint16(img.Pix[i]>>3)<<11 | uint16(img.Pix[i+1]>>2)<<5 | uint16(img.Pix[i+2]>>3)
			binary.LittleEndian.PutUint16(out[(y*revAWidth+x)*2:], pixel)
		}
	}
	return out, nil
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
