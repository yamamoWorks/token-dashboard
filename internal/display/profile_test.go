package display

import (
	"image"
	"testing"
	"time"
)

func TestDisplayProfilesOwnLogicalSizesAndRedrawIntervals(t *testing.T) {
	wantSizes := map[string]image.Point{
		UltraWideProfileID: image.Pt(1920, 462),
		CompactProfileID:   image.Pt(480, 320),
	}
	wantIntervals := map[string]time.Duration{
		UltraWideProfileID: time.Minute,
		CompactProfileID:   compactPageDuration,
	}
	profiles := Profiles()
	if len(profiles) != len(wantSizes) {
		t.Fatalf("profiles = %d, want %d", len(profiles), len(wantSizes))
	}
	for _, profile := range profiles {
		size, ok := wantSizes[profile.ID]
		if !ok {
			t.Fatalf("unexpected profile %q", profile.ID)
		}
		if got := image.Pt(profile.Width, profile.Height); got != size {
			t.Fatalf("profile %q metadata size = %v, want %v", profile.ID, got, size)
		}
		if got := profile.Layout.Size(); got != size {
			t.Fatalf("profile %q layout size = %v, want %v", profile.ID, got, size)
		}
		if got, want := profile.redrawInterval(time.Hour), wantIntervals[profile.ID]; got != want {
			t.Fatalf("profile %q redraw interval = %v, want %v", profile.ID, got, want)
		}
		resolved, ok := ProfileByID(profile.ID)
		if !ok || resolved.ID != profile.ID || image.Pt(resolved.Width, resolved.Height) != size {
			t.Fatalf("resolved profile %q = %+v, ok=%v", profile.ID, resolved, ok)
		}
	}
	if got := DefaultProfile(); got.ID != UltraWideProfileID || image.Pt(got.Width, got.Height) != wantSizes[UltraWideProfileID] {
		t.Fatalf("default profile = %+v", got)
	}
	if _, ok := ProfileByID("unknown"); ok {
		t.Fatal("unknown profile resolved")
	}
}

func TestDisplayProfileRedrawIntervalKeepsFasterApplicationOverride(t *testing.T) {
	compact, ok := ProfileByID(CompactProfileID)
	if !ok {
		t.Fatal("compact profile is missing")
	}
	if got, want := compact.redrawInterval(time.Second), time.Second; got != want {
		t.Fatalf("redraw interval = %v, want %v", got, want)
	}
}
