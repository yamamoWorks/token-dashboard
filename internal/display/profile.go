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

// DisplayProfile binds a stable selection ID to one logical resolution, layout, and maximum redraw
// interval. Transport-specific details are intentionally outside the profile until a device protocol
// is known.
type DisplayProfile struct {
	ID     string
	Name   string
	Width  int
	Height int
	Layout Layout

	maxRedrawInterval time.Duration
}

func newProfile(id, name string, layout Layout, maxRedrawInterval time.Duration) DisplayProfile {
	size := layout.Size()
	return DisplayProfile{
		ID:                id,
		Name:              name,
		Width:             size.X,
		Height:            size.Y,
		Layout:            layout,
		maxRedrawInterval: maxRedrawInterval,
	}
}

// Profiles returns the display profiles the application can render and preview.
func Profiles() []DisplayProfile {
	return []DisplayProfile{
		newProfile(UltraWideProfileID, "TURZX 9.2 Inch", UltraWideLayout{}, time.Minute),
		newProfile(CompactProfileID, "TURZX 3.5 Inch", CompactGaugeLayout{}, compactPageDuration),
	}
}

// DefaultProfile preserves the existing 1920x462 behavior for settings files created before profiles existed.
func DefaultProfile() DisplayProfile {
	return newProfile(UltraWideProfileID, "TURZX 9.2 Inch", UltraWideLayout{}, time.Minute)
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

// redrawInterval returns the shorter of the application's redraw interval and the profile-specific
// maximum. This keeps test/server overrides effective while allowing layouts with time-based content
// to refresh more often in the desktop app.
func (p DisplayProfile) redrawInterval(appInterval time.Duration, paging ...CompactPagingSettings) time.Duration {
	maxInterval := p.maxRedrawInterval
	if _, ok := p.Layout.(CompactGaugeLayout); ok && len(paging) > 0 {
		compact := paging[0].normalized()
		if compact.Auto {
			maxInterval = compact.Interval
		} else {
			maxInterval = 0
		}
	}
	if maxInterval > 0 && maxInterval < appInterval {
		return maxInterval
	}
	return appInterval
}

// Render draws with this profile without mutating the renderer's legacy default layout.
func (p DisplayProfile) Render(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style) *image.RGBA {
	return p.RenderWithPaging(renderer, stats, now, source, style, DefaultCompactPagingSettings())
}

func (p DisplayProfile) RenderWithPaging(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style, paging CompactPagingSettings) *image.RGBA {
	if compact, ok := p.Layout.(CompactGaugeLayout); ok {
		return compact.RenderWithPaging(renderer, stats, now, source, style, paging)
	}
	return p.Layout.Render(renderer, stats, now, source, style)
}
