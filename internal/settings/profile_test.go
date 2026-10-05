package settings

import (
	"errors"
	"image"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	displaypkg "token-monitor-turzx/internal/display"
	"token-monitor-turzx/internal/fault"
	"token-monitor-turzx/internal/turzx"
)

func TestDisplayProfileDefaultsSavesAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	service := New(path, "test.app", func() ([]turzx.Device, error) { return nil, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))

	view, err := service.Get()
	if err != nil {
		t.Fatal(err)
	}
	if view.DisplayProfileID != displaypkg.UltraWideProfileID || len(view.DisplayProfiles) != 2 {
		t.Fatalf("default profile view = %+v", view)
	}
	if got := SelectedDisplayProfile(service); got.ID != displaypkg.UltraWideProfileID || image.Pt(got.Width, got.Height) != image.Pt(1920, 462) {
		t.Fatalf("default selected profile = %+v", got)
	}

	var changes int
	service.OnProfileSaved = func() { changes++ }
	view, err = service.Save(SaveRequest{Source: "Local", DisplayProfileID: displaypkg.CompactProfileID})
	if err != nil {
		t.Fatal(err)
	}
	if view.DisplayProfileID != displaypkg.CompactProfileID || changes != 1 {
		t.Fatalf("saved profile view = %+v, changes = %d", view, changes)
	}
	if got := SelectedDisplayProfile(service); got.ID != displaypkg.CompactProfileID || image.Pt(got.Width, got.Height) != image.Pt(480, 320) {
		t.Fatalf("compact selected profile = %+v", got)
	}
	if _, err := service.Save(SaveRequest{Source: "Local", DisplayProfileID: displaypkg.CompactProfileID}); err != nil || changes != 1 {
		t.Fatalf("saving same profile: err=%v changes=%d", err, changes)
	}

	_, err = service.Save(SaveRequest{Source: "Local", DisplayProfileID: "unknown"})
	var public *fault.Error
	if !errors.As(err, &public) || public.Code != "VALIDATION" || public.FieldErrors["displayProfileID"] == "" {
		t.Fatalf("unknown profile error = %v", err)
	}
	if got := SelectedDisplayProfile(service); got.ID != displaypkg.CompactProfileID {
		t.Fatalf("rejected profile changed selection to %+v", got)
	}
}
