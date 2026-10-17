// Package compact implements the independent 3.5-inch TURZX Rev.A serial protocol.
// It does not modify or use the 9.2-inch WinUSB transport.
package compact

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
)

const (
	Width  = 480
	Height = 320

	stripWidth = 8
	chunkSize  = Width * 8 // Legacy Rev.A serial transfer: 3840 bytes per write.

	// RevAID identifies the one supported Rev.A USB device instance.
	RevAID = `USB\VID_1A86&PID_5722\USB35INCHIPSV2`
)

// Conn writes Rev.A commands to a serial port. Calls must not be concurrent.
// A failed frame transfer closes the port; callers may rediscover and Open it.
type Conn struct {
	port io.WriteCloser
}

// newConn takes ownership of port and sends the landscape initialization.
func newConn(port io.WriteCloser) (*Conn, error) {
	if port == nil {
		return nil, errors.New("TURZX Rev.A serial port is nil")
	}
	c := &Conn{port: port}
	if err := c.write(orientationCommand()); err != nil {
		return nil, errors.Join(fmt.Errorf("initialize TURZX Rev.A landscape: %w", err), c.Close())
	}
	return c, nil
}

func orientationCommand() []byte {
	cmd := make([]byte, 16)
	cmd[5] = 121
	cmd[6] = 102 // Landscape orientation 2, encoded with +100.
	cmd[7], cmd[8] = byte(Width>>8), byte(Width&0xff)
	cmd[9], cmd[10] = byte(Height>>8), byte(Height&0xff)
	return cmd
}

func resetCommand() []byte { return []byte{0, 0, 0, 0, 0, 101} }

// bitmapCommand encodes an inclusive rectangle using the Rev.A six-byte header.
func bitmapCommand(x, y, ex, ey int) ([]byte, error) {
	if x < 0 || y < 0 || ex < x || ey < y || ex >= Width || ey >= Height {
		return nil, fmt.Errorf("TURZX Rev.A invalid bitmap coordinates (%d,%d)-(%d,%d)", x, y, ex, ey)
	}
	return []byte{byte(x >> 2), byte(((x & 3) << 6) | (y >> 4)), byte(((y & 15) << 4) | (ex >> 6)), byte(((ex & 63) << 2) | (ey >> 8)), byte(ey), 197}, nil
}

func checkFrame(img *image.RGBA) error {
	if img == nil {
		return errors.New("TURZX Rev.A frame is nil")
	}
	if img.Bounds().Dx() != Width || img.Bounds().Dy() != Height {
		return fmt.Errorf("TURZX Rev.A frame must be %dx%d, got %dx%d", Width, Height, img.Bounds().Dx(), img.Bounds().Dy())
	}
	return nil
}

// rgb565LERect converts a frame-relative rectangle in row-major order.
// A frame with non-zero bounds (including a SubImage) is supported.
func rgb565LERect(img *image.RGBA, r image.Rectangle) ([]byte, error) {
	if err := checkFrame(img); err != nil {
		return nil, err
	}
	if r.Empty() || !r.In(image.Rect(0, 0, Width, Height)) {
		return nil, fmt.Errorf("TURZX Rev.A rectangle %v is outside the frame", r)
	}
	pixels := make([]byte, r.Dx()*r.Dy()*2)
	bounds := img.Bounds()
	i := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			offset := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			value := uint16(img.Pix[offset]>>3)<<11 | uint16(img.Pix[offset+1]>>2)<<5 | uint16(img.Pix[offset+2]>>3)
			binary.LittleEndian.PutUint16(pixels[i:], value)
			i += 2
		}
	}
	return pixels, nil
}

// strips returns exactly 60 nonoverlapping vertical, full-height strips left to right.
func strips() []image.Rectangle {
	rects := make([]image.Rectangle, 0, Width/stripWidth)
	for x := 0; x < Width; x += stripWidth {
		rects = append(rects, image.Rect(x, 0, x+stripWidth, Height))
	}
	return rects
}

func chunks(data []byte) [][]byte {
	out := make([][]byte, 0, (len(data)+chunkSize-1)/chunkSize)
	for len(data) > 0 {
		n := min(len(data), chunkSize)
		out = append(out, data[:n])
		data = data[n:]
	}
	return out
}

func (c *Conn) write(data []byte) error {
	if c == nil || c.port == nil {
		return errors.New("TURZX Rev.A serial connection is closed")
	}
	for len(data) > 0 {
		n, err := c.port.Write(data)
		if n < 0 || n > len(data) {
			return fmt.Errorf("TURZX Rev.A serial write reported invalid count %d", n)
		}
		if err != nil {
			return fmt.Errorf("TURZX Rev.A serial write: %w", err)
		}
		if n == 0 {
			return errors.New("TURZX Rev.A serial write made no progress")
		}
		data = data[n:]
	}
	return nil
}

func (c *Conn) sendFailed(err error) error {
	return errors.Join(err, c.Close())
}

// SendFrame sends RGB565LE strips and inclusive bitmap commands in left-to-right
// order. Any I/O failure closes the port; the next Open rediscovers its COM name.
func (c *Conn) SendFrame(img *image.RGBA) error {
	if err := checkFrame(img); err != nil {
		return err
	}
	for i, rect := range strips() {
		pixels, err := rgb565LERect(img, rect)
		if err != nil {
			return err
		}
		command, err := bitmapCommand(rect.Min.X, rect.Min.Y, rect.Max.X-1, rect.Max.Y-1)
		if err != nil {
			return err
		}
		if err := c.write(command); err != nil {
			return c.sendFailed(fmt.Errorf("TURZX Rev.A strip %d command: %w", i, err))
		}
		for _, chunk := range chunks(pixels) {
			if err := c.write(chunk); err != nil {
				return c.sendFailed(fmt.Errorf("TURZX Rev.A strip %d pixels: %w", i, err))
			}
		}
	}
	return nil
}

// Reset restores the startup screen with the legacy Rev.A RESET command.
func (c *Conn) Reset() error {
	if err := c.write(resetCommand()); err != nil {
		return c.sendFailed(fmt.Errorf("TURZX Rev.A reset: %w", err))
	}
	return nil
}

// Exit is the legacy sender's exit operation. Call Close afterward to release
// the handle, or call Shutdown to do both in one operation.
func (c *Conn) Exit() error { return c.Reset() }

// Shutdown sends RESET, then closes the serial port even when RESET fails.
func (c *Conn) Shutdown() error {
	return errors.Join(c.Reset(), c.Close())
}

// Close releases the COM handle without sending commands. It is idempotent.
func (c *Conn) Close() error {
	if c == nil || c.port == nil {
		return nil
	}
	port := c.port
	c.port = nil
	return port.Close()
}
