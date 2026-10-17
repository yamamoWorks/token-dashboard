package compact

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
)

type fakePort struct {
	data       []byte
	writes     [][]byte
	maxWrite   int
	failAt     int
	failErr    error
	zeroAt     int
	closeCount int
	closeErr   error
}

func (p *fakePort) Write(b []byte) (int, error) {
	call := len(p.writes) + 1
	p.writes = append(p.writes, append([]byte(nil), b...))
	if call == p.failAt {
		return 0, p.failErr
	}
	if call == p.zeroAt {
		return 0, nil
	}
	n := len(b)
	if p.maxWrite > 0 && n > p.maxWrite {
		n = p.maxWrite
	}
	p.data = append(p.data, b[:n]...)
	return n, nil
}

func (p *fakePort) Close() error { p.closeCount++; return p.closeErr }

func TestCommands(t *testing.T) {
	if got, want := resetCommand(), []byte{0, 0, 0, 0, 0, 101}; !bytes.Equal(got, want) {
		t.Fatalf("reset = % x, want % x", got, want)
	}
	if got, want := orientationCommand(), []byte{0, 0, 0, 0, 0, 121, 102, 1, 224, 1, 64, 0, 0, 0, 0, 0}; !bytes.Equal(got, want) {
		t.Fatalf("orientation = % x, want % x", got, want)
	}
	for _, tc := range []struct {
		x, y, ex, ey int
		want         []byte
	}{
		{0, 0, 479, 319, []byte{0, 0, 7, 0x7d, 0x3f, 197}},
		{0, 0, 7, 319, []byte{0, 0, 0, 0x1d, 0x3f, 197}},
		{8, 0, 15, 319, []byte{2, 0, 0, 0x3d, 0x3f, 197}},
		{472, 0, 479, 319, []byte{0x76, 0, 7, 0x7d, 0x3f, 197}},
		{5, 37, 300, 200, []byte{1, 0x42, 0x54, 0xb0, 0xc8, 197}},
	} {
		got, err := bitmapCommand(tc.x, tc.y, tc.ex, tc.ey)
		if err != nil || !bytes.Equal(got, tc.want) {
			t.Errorf("bitmap (%d,%d)-(%d,%d) = % x, %v; want % x", tc.x, tc.y, tc.ex, tc.ey, got, err, tc.want)
		}
	}
	for _, rect := range [][4]int{{-1, 0, 7, 319}, {0, 0, 480, 319}, {0, 0, 7, 320}, {10, 0, 9, 0}, {0, -1, 1, 1}} {
		if _, err := bitmapCommand(rect[0], rect[1], rect[2], rect[3]); err == nil {
			t.Errorf("accepted bitmap %v", rect)
		}
	}
}

func TestStripsAndChunks(t *testing.T) {
	rects := strips()
	if len(rects) != 60 {
		t.Fatalf("strip count = %d", len(rects))
	}
	for i, r := range rects {
		if want := image.Rect(i*8, 0, (i+1)*8, Height); r != want {
			t.Fatalf("strip %d = %v, want %v", i, r, want)
		}
	}
	data := make([]byte, stripWidth*Height*2)
	parts := chunks(data)
	if len(parts) != 2 || len(parts[0]) != 3840 || len(parts[1]) != 1280 {
		t.Fatalf("unexpected chunk lengths")
	}
	if len(chunks(nil)) != 0 {
		t.Fatal("empty image produced nonempty chunks")
	}
}

func testFrame(min image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rect(min.X, min.Y, min.X+Width, min.Y+Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			img.SetRGBA(min.X+x, min.Y+y, color.RGBA{uint8(x * 7), uint8(y * 3), uint8(x + y), 255})
		}
	}
	return img
}

func TestRGB565ColorsAndBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for x, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}} {
		img.SetRGBA(x, 0, c)
	}
	got, err := rgb565LERect(img, image.Rect(0, 0, 4, 1))
	if err != nil || !bytes.Equal(got, []byte{0, 0xf8, 0xe0, 7, 0x1f, 0, 0xff, 0xff}) {
		t.Fatalf("known colors = % x, %v", got, err)
	}
	base := image.NewRGBA(image.Rect(0, 0, 510, 350))
	sub := base.SubImage(image.Rect(10, 20, 490, 340)).(*image.RGBA)
	for y := range Height {
		for x := range Width {
			sub.SetRGBA(10+x, 20+y, color.RGBA{uint8(x * 7), uint8(y * 3), uint8(x + y), 255})
		}
	}
	expected := testFrame(image.Point{})
	for _, r := range []image.Rectangle{image.Rect(0, 0, 8, 320), image.Rect(472, 0, 480, 320), image.Rect(3, 319, 11, 320)} {
		got, err := rgb565LERect(sub, r)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := rgb565LERect(expected, r)
		if !bytes.Equal(got, want) {
			t.Fatalf("subimage bytes differ for %v", r)
		}
	}
	for _, tc := range []struct {
		img *image.RGBA
		r   image.Rectangle
	}{
		{nil, image.Rect(0, 0, 8, Height)},
		{image.NewRGBA(image.Rect(0, 0, Height, Width)), image.Rect(0, 0, 8, Height)},
		{img, image.Rect(-1, 0, 8, Height)},
		{img, image.Rect(476, 0, 484, Height)},
		{img, image.Rect(0, 0, 8, Height+1)},
		{img, image.Rect(8, 0, 8, Height)},
	} {
		if _, err := rgb565LERect(tc.img, tc.r); err == nil {
			t.Fatalf("accepted invalid frame or rectangle: %v", tc.r)
		}
	}
}

func TestSendFrameSequenceAndLifecycle(t *testing.T) {
	port := &fakePort{}
	conn, err := newConn(port)
	if err != nil {
		t.Fatal(err)
	}
	frame := testFrame(image.Point{})
	if err := conn.SendFrame(frame); err != nil {
		t.Fatal(err)
	}
	if got, want := len(port.data), 16+60*(6+5120); got != want {
		t.Fatalf("byte count = %d, want %d", got, want)
	}
	if got := len(port.writes); got != 1+60*3 {
		t.Fatalf("write count = %d, want 181", got)
	}
	if !bytes.Equal(port.writes[0], orientationCommand()) {
		t.Fatal("missing landscape initialization")
	}
	for i, r := range strips() {
		at := 1 + i*3
		cmd, _ := bitmapCommand(r.Min.X, 0, r.Max.X-1, Height-1)
		if !bytes.Equal(port.writes[at], cmd) {
			t.Fatalf("strip %d command differs", i)
		}
		if len(port.writes[at+1]) != 3840 || len(port.writes[at+2]) != 1280 {
			t.Fatalf("strip %d chunks lengths = %d, %d", i, len(port.writes[at+1]), len(port.writes[at+2]))
		}
		pixels, _ := rgb565LERect(frame, r)
		if !bytes.Equal(append(append([]byte{}, port.writes[at+1]...), port.writes[at+2]...), pixels) {
			t.Fatalf("strip %d bytes differ", i)
		}
	}
	if err := conn.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(port.data, resetCommand()) || port.closeCount != 1 {
		t.Fatalf("shutdown did not reset/close: close count %d", port.closeCount)
	}
	if err := conn.Close(); err != nil || port.closeCount != 1 {
		t.Fatalf("Close not idempotent: %v, count %d", err, port.closeCount)
	}
}

func TestFailuresCloseAndPropagate(t *testing.T) {
	if c, err := newConn(nil); c != nil || err == nil {
		t.Fatal("accepted nil serial port")
	}
	for _, tc := range []struct {
		name     string
		port     *fakePort
		duringOp bool
	}{
		{"initialization error", &fakePort{failAt: 1, failErr: errors.New("unplugged")}, false},
		{"initialization zero progress", &fakePort{zeroAt: 1}, false},
		{"command error", &fakePort{failAt: 2, failErr: errors.New("unplugged")}, true},
		{"data error", &fakePort{failAt: 3, failErr: errors.New("unplugged")}, true},
		{"data zero progress", &fakePort{zeroAt: 3}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := newConn(tc.port)
			if tc.duringOp {
				if err != nil {
					t.Fatal(err)
				}
				err = conn.SendFrame(testFrame(image.Point{}))
			}
			if err == nil || tc.port.closeCount != 1 {
				t.Fatalf("error %v, close count %d", err, tc.port.closeCount)
			}
			if tc.duringOp {
				if err := conn.SendFrame(testFrame(image.Point{})); err == nil {
					t.Fatal("reused failed/closed serial port")
				}
			}
		})
	}
	port := &fakePort{maxWrite: 2}
	conn, err := newConn(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Exit(); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(port.data, resetCommand()) {
		t.Fatal("partial writes did not complete the reset command")
	}
	conn.Close()
	port = &fakePort{failAt: 2, failErr: errors.New("reset failed")}
	conn, err = newConn(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Shutdown(); err == nil || !strings.Contains(err.Error(), "reset failed") || port.closeCount != 1 {
		t.Fatalf("shutdown failure: err %v, close count %d", err, port.closeCount)
	}
}

func TestSendFrameRejectsInvalidWithoutWriting(t *testing.T) {
	port := &fakePort{}
	conn, err := newConn(port)
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range []*image.RGBA{nil, image.NewRGBA(image.Rect(0, 0, Height, Width))} {
		if err := conn.SendFrame(frame); err == nil {
			t.Fatal("accepted invalid frame")
		}
	}
	if len(port.writes) != 1 {
		t.Fatalf("invalid frame wrote %d extra calls", len(port.writes)-1)
	}
	conn.Close()
}
