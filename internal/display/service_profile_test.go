package display

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"token-monitor-turzx/internal/usage"
)

type solidProfileLayout struct{ size image.Point }

func (l solidProfileLayout) Size() image.Point { return l.size }
func (l solidProfileLayout) Render(_ *Renderer, _ *usage.Stats, _ time.Time, _ string, _ Style) *image.RGBA {
	return image.NewRGBA(image.Rectangle{Max: l.size})
}

func testProfileService(state *usage.State, logger *slog.Logger) *Service {
	return &Service{
		State:  state,
		Hidden: func() ([]string, error) { return nil, nil },
		Show:   func([]string, bool) error { return nil },
		Logger: logger,
	}
}

func TestRunPreviewUsesSelectedProfileSize(t *testing.T) {
	compact, ok := ProfileByID(CompactProfileID)
	if !ok {
		t.Fatal("compact profile is missing")
	}
	// Keep the built-in profile metadata while replacing only drawing with a font-free test layout.
	compact.Layout = solidProfileLayout{size: image.Pt(compact.Width, compact.Height)}

	state := usage.NewState()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := testProfileService(state, logger)
	updated := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, service, &Renderer{}, state, time.Hour,
			func() DisplayProfile { return compact }, func() Style { return Gauges },
			func(*image.RGBA) {}, func(name string, _ any) {
				if name == Updated {
					updated <- struct{}{}
				}
			}, logger)
	}()

	select {
	case <-updated:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("preview was not updated")
	}
	cancel()
	<-done

	encoded := strings.TrimPrefix(service.Preview(), "data:image/png;base64,")
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := image.Pt(config.Width, config.Height), image.Pt(480, 320); got != want {
		t.Fatalf("preview size = %v, want %v", got, want)
	}
}

func TestRunUsesSelectedProfileRedrawInterval(t *testing.T) {
	compact, ok := ProfileByID(CompactProfileID)
	if !ok {
		t.Fatal("compact profile is missing")
	}
	compact.Layout = solidProfileLayout{size: image.Pt(compact.Width, compact.Height)}
	compact.maxRedrawInterval = 20 * time.Millisecond

	state := usage.NewState()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := testProfileService(state, logger)
	rendered := make(chan struct{}, 3)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, service, &Renderer{}, state, time.Hour,
			func() DisplayProfile { return compact }, func() Style { return Gauges },
			func(*image.RGBA) { rendered <- struct{}{} }, func(string, any) {}, logger)
	}()

	for range 2 {
		select {
		case <-rendered:
		case <-time.After(500 * time.Millisecond):
			cancel()
			<-done
			t.Fatal("profile-specific redraw interval was not used")
		}
	}
	cancel()
	<-done
}

func TestPreviewPageClampsSelectionAndReportsPageCount(t *testing.T) {
	service := &Service{
		preview:      "device",
		previewPages: []string{"page-1", "page-2", "page-3"},
	}

	first := service.PreviewPage(-1)
	if first.Image != "page-1" || first.Page != 0 || first.PageCount != 3 {
		t.Fatalf("first preview = %+v", first)
	}
	last := service.PreviewPage(99)
	if last.Image != "page-3" || last.Page != 2 || last.PageCount != 3 {
		t.Fatalf("last preview = %+v", last)
	}
}

func TestPreviewPageFallsBackToDeviceImageBeforePagedPreviewExists(t *testing.T) {
	service := &Service{preview: "device"}
	got := service.PreviewPage(3)
	if got.Image != "device" || got.Page != 0 || got.PageCount != 1 {
		t.Fatalf("preview = %+v", got)
	}
}
