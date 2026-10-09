package turzx

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestRevABitmapCommand(t *testing.T) {
	tests := []struct {
		name         string
		x, y, ex, ey int
		want         []byte
	}{
		{"full screen", 0, 0, 479, 319, []byte{0x00, 0x00, 0x07, 0x7d, 0x3f, 197}},
		{"first strip", 0, 0, 7, 319, []byte{0x00, 0x00, 0x00, 0x1d, 0x3f, 197}},
		{"second strip", 8, 0, 15, 319, []byte{0x02, 0x00, 0x00, 0x3d, 0x3f, 197}},
		{"last strip", 472, 0, 479, 319, []byte{0x76, 0x00, 0x07, 0x7d, 0x3f, 197}},
		{"odd rect", 5, 37, 300, 200, []byte{0x01, 0x42, 0x54, 0xb0, 0xc8, 197}},
	}
	for _, tt := range tests {
		if got := revABitmapCommand(tt.x, tt.y, tt.ex, tt.ey); !bytes.Equal(got, tt.want) {
			t.Errorf("%s: command = % x, want % x", tt.name, got, tt.want)
		}
	}
}

func TestRevARects(t *testing.T) {
	rects := revARects()
	if len(rects) != 60 {
		t.Fatalf("got %d rects, want 60", len(rects))
	}
	next := 0
	for i, r := range rects {
		if r.Min.X != next || r.Dx() != 8 || r.Min.Y != 0 || r.Max.Y != revAHeight {
			t.Fatalf("rect %d = %v, want x=%d width 8 full height", i, r, next)
		}
		next = r.Max.X
	}
	if next != revAWidth {
		t.Fatalf("rects end at x=%d, want %d", next, revAWidth)
	}
}

func revATestPixel(x, y int) color.RGBA {
	return color.RGBA{uint8(x * 7), uint8(y * 3), uint8(x + y), 255}
}

func revATestRGB565(c color.RGBA) []byte {
	v := uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
	return []byte{byte(v), byte(v >> 8)}
}

// revATestStrip builds the expected bytes of r from revATestPixel, row-major within r.
func revATestStrip(r image.Rectangle) []byte {
	var out []byte
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			out = append(out, revATestRGB565(revATestPixel(x, y))...)
		}
	}
	return out
}

// revATestFrame returns a frame whose pixel at logical (x, y) is revATestPixel(x, y).
func revATestFrame(min image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rect(min.X, min.Y, min.X+revAWidth, min.Y+revAHeight))
	for y := 0; y < revAHeight; y++ {
		for x := 0; x < revAWidth; x++ {
			img.SetRGBA(min.X+x, min.Y+y, revATestPixel(x, y))
		}
	}
	return img
}

func TestRevARGB565LERectColors(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, revAWidth, revAHeight))
	for x, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}} {
		img.SetRGBA(x, 0, c)
	}
	got, err := revARGB565LERect(img, image.Rect(0, 0, 4, 1))
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x00, 0xf8, 0xe0, 0x07, 0x1f, 0x00, 0xff, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("colors = % x, want % x", got, want)
	}
}

func TestRevARGB565LERectLayout(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for y := 0; y < revAHeight; y++ {
		for x := 0; x < revAWidth; x++ {
			base.SetRGBA(10+x, 20+y, revATestPixel(x, y))
		}
	}
	frames := map[string]*image.RGBA{
		"zero origin": revATestFrame(image.Point{}),
		"offset":      revATestFrame(image.Point{10, 20}),
		"sub image":   base.SubImage(image.Rect(10, 20, 10+revAWidth, 20+revAHeight)).(*image.RGBA),
	}
	rects := []image.Rectangle{image.Rect(0, 0, 8, 320), image.Rect(8, 0, 16, 320), image.Rect(472, 0, 480, 320), image.Rect(3, 319, 11, 320)}
	for name, img := range frames {
		for _, r := range rects {
			got, err := revARGB565LERect(img, r)
			if err != nil {
				t.Fatalf("%s %v: %v", name, r, err)
			}
			if want := revATestStrip(r); !bytes.Equal(got, want) {
				t.Errorf("%s %v: pixel bytes differ (len %d, want %d)", name, r, len(got), len(want))
			}
		}
	}
	if got, _ := revARGB565LERect(frames["zero origin"], revARects()[0]); len(got) != 5120 {
		t.Fatalf("strip length = %d, want 5120", len(got))
	}
}

func TestRevARGB565LERectErrors(t *testing.T) {
	ok := image.NewRGBA(image.Rect(0, 0, revAWidth, revAHeight))
	strip := image.Rect(0, 0, 8, revAHeight)
	tests := []struct {
		name string
		img  *image.RGBA
		r    image.Rectangle
	}{
		{"nil frame", nil, strip},
		{"wrong size", image.NewRGBA(image.Rect(0, 0, 320, 480)), strip},
		{"outside right", ok, image.Rect(476, 0, 484, revAHeight)},
		{"outside bottom", ok, image.Rect(0, 0, 8, revAHeight+1)},
		{"negative", ok, image.Rect(-1, 0, 7, revAHeight)},
		{"empty", ok, image.Rect(8, 0, 8, revAHeight)},
	}
	for _, tt := range tests {
		if got, err := revARGB565LERect(tt.img, tt.r); err == nil || got != nil {
			t.Errorf("%s: got %d bytes, err %v; want error", tt.name, len(got), err)
		}
	}
}
