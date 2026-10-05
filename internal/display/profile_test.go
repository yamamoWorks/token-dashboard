package display

import (
	"image"
	"testing"
)

func TestDisplayProfilesOwnLogicalSizes(t *testing.T) {
	want := map[string]image.Point{
		UltraWideProfileID: image.Pt(1920, 462),
		CompactProfileID:   image.Pt(480, 320),
	}
	profiles := Profiles()
	if len(profiles) != len(want) {
		t.Fatalf("profiles = %d, want %d", len(profiles), len(want))
	}
	for _, profile := range profiles {
		size, ok := want[profile.ID]
		if !ok {
			t.Fatalf("unexpected profile %q", profile.ID)
		}
		if got := image.Pt(profile.Width, profile.Height); got != size {
			t.Fatalf("profile %q metadata size = %v, want %v", profile.ID, got, size)
		}
		if got := profile.Layout.Size(); got != size {
			t.Fatalf("profile %q layout size = %v, want %v", profile.ID, got, size)
		}
		resolved, ok := ProfileByID(profile.ID)
		if !ok || resolved.ID != profile.ID || image.Pt(resolved.Width, resolved.Height) != size {
			t.Fatalf("resolved profile %q = %+v, ok=%v", profile.ID, resolved, ok)
		}
	}
	if got := DefaultProfile(); got.ID != UltraWideProfileID || image.Pt(got.Width, got.Height) != want[UltraWideProfileID] {
		t.Fatalf("default profile = %+v", got)
	}
	if _, ok := ProfileByID("unknown"); ok {
		t.Fatal("unknown profile resolved")
	}
}
