package compact

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func encodedCanvas(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	src := image.NewRGBA(image.Rect(0, 0, sourceWidth, sourceHeight))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	var b bytes.Buffer
	if err := png.Encode(&b, src); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFrameFromPNGLeftTopCropAndColors(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, sourceWidth, sourceHeight))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: color.RGBA{R: 1, G: 2, B: 3, A: 255}}, image.Point{}, draw.Src)
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	src.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	src.SetRGBA(8, 1, color.RGBA{B: 255, A: 255})
	src.SetRGBA(479, 319, color.RGBA{255, 255, 255, 255})
	src.SetRGBA(480, 0, color.RGBA{255, 255, 0, 255})
	src.SetRGBA(0, 320, color.RGBA{255, 0, 255, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, src); err != nil {
		t.Fatal(err)
	}
	frame, err := frameFromImage(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if frame.Bounds() != image.Rect(0, 0, Width, Height) {
		t.Fatalf("unexpected crop bounds %v", frame.Bounds())
	}
	for _, tc := range []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{R: 255, A: 255}},
		{1, 0, color.RGBA{G: 255, A: 255}},
		{8, 1, color.RGBA{B: 255, A: 255}},
		{479, 319, color.RGBA{255, 255, 255, 255}},
		{479, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255}},
		{0, 319, color.RGBA{R: 1, G: 2, B: 3, A: 255}},
	} {
		if got := frame.RGBAAt(tc.x, tc.y); got != tc.want {
			t.Errorf("crop (%d,%d) = %+v, want %+v", tc.x, tc.y, got, tc.want)
		}
	}
	pixels, err := rgb565LERect(frame, image.Rect(0, 0, 2, 1))
	if err != nil || !bytes.Equal(pixels, []byte{0, 0xf8, 0xe0, 7}) {
		t.Fatalf("RGB565LE = % x, %v", pixels, err)
	}
	port := &fakePort{}
	conn, err := newConn(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SendFrame(frame); err != nil {
		t.Fatal(err)
	}
	if got := port.data[16+6 : 16+6+4]; !bytes.Equal(got, pixels) {
		t.Fatalf("serial pixels = % x, want % x", got, pixels)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestJPEGBlackBecomesOpaqueBlackFrame(t *testing.T) {
	// This matches display.black(): a 462x1920 grayscale JPEG.
	src := image.NewGray(image.Rect(0, 0, sourceHeight, sourceWidth))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, src, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	frame, err := frameFromImage(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if frame.Bounds() != image.Rect(0, 0, Width, Height) {
		t.Fatalf("wrong black dimensions: %v", frame.Bounds())
	}
	for i := 0; i < len(frame.Pix); i += 4 {
		if !bytes.Equal(frame.Pix[i:i+4], []byte{0, 0, 0, 255}) {
			t.Fatalf("non-black pixel at offset %d: % x", i, frame.Pix[i:i+4])
		}
	}
	if _, err := rgb565LERect(frame, image.Rect(0, 0, Width, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestRejectMalformedUnexpectedAndNonblackFrames(t *testing.T) {
	var wrongPNG bytes.Buffer
	if err := png.Encode(&wrongPNG, image.NewRGBA(image.Rect(0, 0, Width, Height))); err != nil {
		t.Fatal(err)
	}
	var wrongJPEG bytes.Buffer
	if err := jpeg.Encode(&wrongJPEG, image.NewGray(image.Rect(0, 0, Width, Height)), nil); err != nil {
		t.Fatal(err)
	}
	nonBlack := image.NewGray(image.Rect(0, 0, sourceHeight, sourceWidth))
	draw.Draw(nonBlack, image.Rect(0, 0, 16, 16), &image.Uniform{C: color.Gray{Y: 255}}, image.Point{}, draw.Src)
	var nonBlackJPEG bytes.Buffer
	if err := jpeg.Encode(&nonBlackJPEG, nonBlack, nil); err != nil {
		t.Fatal(err)
	}
	validPNG := encodedCanvas(t, color.RGBA{A: 255})
	tests := []struct {
		name string
		data []byte
		part string
	}{
		{"empty", nil, "header"},
		{"unknown", []byte("GIF89a"), "header"},
		{"invalid dimensions PNG", wrongPNG.Bytes(), "must be"},
		{"invalid dimensions JPEG", wrongJPEG.Bytes(), "must be"},
		{"unexpected picture JPEG", nonBlackJPEG.Bytes(), "non-black"},
		{"truncated PNG", validPNG[:len(validPNG)-5], "decode compact PNG"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := frameFromImage(tc.data); err == nil || !strings.Contains(err.Error(), tc.part) {
				t.Fatalf("accepted invalid image or wrong error: %v", err)
			}
		})
	}
}
