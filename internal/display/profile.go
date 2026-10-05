package display

import (
	"image"
	"time"

	"token-monitor-turzx/internal/usage"
)

const (
	UltraWideProfileID = "ultra-wide-1920x462"
	CompactProfileID   = "compact-gauge-480x320"
)

// DisplayProfile binds a stable selection ID to one logical resolution and layout.
// Transport-specific details are intentionally outside the profile until a device protocol is known.
type DisplayProfile struct {
	ID     string
	Name   string
	Width  int
	Height int
	Layout Layout
}

func newProfile(id, name string, layout Layout) DisplayProfile {
	size := layout.Size()
	return DisplayProfile{ID: id, Name: name, Width: size.X, Height: size.Y, Layout: layout}
}

// Profiles returns the display profiles the application can render and preview.
func Profiles() []DisplayProfile {
	return []DisplayProfile{
		newProfile(UltraWideProfileID, "1920 × 462 Ultra-wide", UltraWideLayout{}),
		newProfile(CompactProfileID, "480 × 320 Compact Gauge", CompactGaugeLayout{}),
	}
}

// DefaultProfile preserves the existing 1920x462 behavior for settings files created before profiles existed.
func DefaultProfile() DisplayProfile {
	return newProfile(UltraWideProfileID, "1920 × 462 Ultra-wide", UltraWideLayout{})
}

// ProfileByID resolves one selectable profile.
func ProfileByID(id string) (DisplayProfile, bool) {
	for _, profile := range Profiles() {
		if profile.ID == id {
			return profile, true
		}
	}
	return DisplayProfile{}, false
}

// Render draws with this profile without mutating the renderer's legacy default layout.
func (p DisplayProfile) Render(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style) *image.RGBA {
	return p.Layout.Render(renderer, stats, now, source, style)
}
