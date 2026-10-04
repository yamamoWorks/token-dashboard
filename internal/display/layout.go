package display

import (
	"image"
	"image/draw"
	"time"

	"token-monitor-turzx/internal/usage"
)

// Layout owns the logical display size and the placement policy for one device profile.
// Drawing primitives remain in package display so additional layouts can reuse them without
// changing the existing ultra-wide implementation.
type Layout interface {
	Size() image.Point
	Render(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style) *image.RGBA
}

// UltraWideLayout is the existing 1920x462 TURZX layout.
type UltraWideLayout struct{}

const (
	ultraWideWidth  = 1920
	ultraWideHeight = 462

	// Width and Height are used only by the existing ultra-wide drawing helpers. New layouts must
	// take their logical dimensions from Layout.Size instead of depending on these values.
	Width  = ultraWideWidth
	Height = ultraWideHeight
)

func (UltraWideLayout) Size() image.Point {
	return image.Pt(ultraWideWidth, ultraWideHeight)
}

func (l UltraWideLayout) Render(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style) *image.RGBA {
	size := l.Size()
	img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	if stats == nil {
		face := renderer.face(false, 64)
		label := "Waiting for Hub"
		if source == "Local" {
			label = "Waiting for local usage"
		}
		renderer.text(img, face, dim, (size.X-measure(face, label))/2, size.Y/2+22, label)
		return img
	}
	// Gauges use the whole width for Usage Limits and leave Tokens out.
	if style == Bars {
		renderer.tokens(img, stats.Periods)
		renderer.limits(img, stats.Limits, now)
	} else {
		renderer.gauges(img, stats, now)
	}
	return img
}
