package settings

import (
	"errors"
	"image"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

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

func TestCompactPagingDefaultsSavesValidatesAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	newService := func() *Service {
		return New(path, "test.app", func() ([]turzx.Device, error) { return nil, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	service := newService()

	view, err := service.Get()
	if err != nil {
		t.Fatal(err)
	}
	if !view.CompactAutoPage || view.CompactPageIntervalSeconds != 10 {
		t.Fatalf("default compact paging view = %+v", view)
	}
	if got := CompactPaging(service); got != (displaypkg.CompactPagingSettings{Auto: true, Interval: 10 * time.Second}) {
		t.Fatalf("default compact paging = %+v", got)
	}

	var changes int
	service.OnPagingSaved = func() { changes++ }
	view, err = service.SaveCompactPaging(CompactPagingRequest{Auto: false, IntervalSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	if view.CompactAutoPage || view.CompactPageIntervalSeconds != 30 || changes != 1 {
		t.Fatalf("disabled compact paging view = %+v, changes = %d", view, changes)
	}
	if got := CompactPaging(service); got != (displaypkg.CompactPagingSettings{Auto: false, Interval: 30 * time.Second}) {
		t.Fatalf("disabled compact paging = %+v", got)
	}

	// Saving the same values again does not request another redraw.
	if _, err := service.SaveCompactPaging(CompactPagingRequest{Auto: false, IntervalSeconds: 30}); err != nil || changes != 1 {
		t.Fatalf("saving same compact paging: err=%v changes=%d", err, changes)
	}

	// Turning automatic paging back on retains the previously selected interval.
	view, err = service.SaveCompactPaging(CompactPagingRequest{Auto: true, IntervalSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !view.CompactAutoPage || view.CompactPageIntervalSeconds != 30 || changes != 2 {
		t.Fatalf("enabled compact paging view = %+v, changes = %d", view, changes)
	}

	reloaded := newService()
	view, err = reloaded.Get()
	if err != nil {
		t.Fatal(err)
	}
	if !view.CompactAutoPage || view.CompactPageIntervalSeconds != 30 {
		t.Fatalf("reloaded compact paging view = %+v", view)
	}

	_, err = reloaded.SaveCompactPaging(CompactPagingRequest{Auto: true, IntervalSeconds: 7})
	var public *fault.Error
	if !errors.As(err, &public) || public.Code != "VALIDATION" || public.FieldErrors["compactPageIntervalSeconds"] == "" {
		t.Fatalf("invalid compact interval error = %v", err)
	}
	if got := CompactPaging(reloaded); got != (displaypkg.CompactPagingSettings{Auto: true, Interval: 30 * time.Second}) {
		t.Fatalf("rejected compact interval changed settings to %+v", got)
	}
}
