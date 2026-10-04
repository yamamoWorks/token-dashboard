package display

import (
	"image"
	"testing"
	"time"

	"token-monitor-turzx/internal/usage"
)

type recordingLayout struct {
	size  image.Point
	calls int
}

func (l *recordingLayout) Size() image.Point { return l.size }

func (l *recordingLayout) Render(_ *Renderer, _ *usage.Stats, _ time.Time, _ string, _ Style) *image.RGBA {
	l.calls++
	return image.NewRGBA(image.Rectangle{Max: l.size})
}

func TestRendererDelegatesToLayout(t *testing.T) {
	layout := &recordingLayout{size: image.Pt(480, 320)}
	renderer := &Renderer{layout: layout}

	img := renderer.Render(&usage.Stats{}, time.Unix(0, 0), "Hub", Bars)
	if layout.calls != 1 {
		t.Fatalf("layout render calls = %d, want 1", layout.calls)
	}
	if got := renderer.Size(); got != layout.size {
		t.Fatalf("renderer size = %v, want %v", got, layout.size)
	}
	if got := img.Bounds().Size(); got != layout.size {
		t.Fatalf("rendered size = %v, want %v", got, layout.size)
	}
}

func TestUltraWideLayoutSize(t *testing.T) {
	if got, want := (UltraWideLayout{}).Size(), image.Pt(1920, 462); got != want {
		t.Fatalf("ultra-wide size = %v, want %v", got, want)
	}
}

func TestNewRendererWithLayoutRejectsNil(t *testing.T) {
	if _, err := NewRendererWithLayout(nil); err == nil {
		t.Fatal("NewRendererWithLayout(nil) succeeded")
	}
}
