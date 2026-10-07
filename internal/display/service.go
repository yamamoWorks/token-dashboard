package display

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"log/slog"
	"sync"
	"time"

	"token-monitor-turzx/internal/fault"
	"token-monitor-turzx/internal/usage"
)

// Updated is emitted after a new image is available from Preview.
const Updated = "display:updated"

// Service hands the latest image to the window. The window never draws it.
type Service struct {
	// State is where Limits reads the contracts from; SetShown asks it for a redraw.
	State *usage.State
	// Hidden returns the saved keys of the windows that are not drawn, and Show saves the change.
	Hidden func() ([]string, error)
	Show   func(keys []string, shown bool) error
	Paging func() CompactPagingSettings
	Logger *slog.Logger

	mu           sync.Mutex
	preview      string
	previewPages []string
}

type PreviewFrame struct {
	Image     string `json:"image"`
	Page      int    `json:"page"`
	PageCount int    `json:"pageCount"`
}

type previewPager interface {
	previewPages(renderer *Renderer, stats *usage.Stats, now time.Time, source string, style Style) []*image.RGBA
}

// hiddenSet is the saved hidden windows as a set.
func (s *Service) hiddenSet() (map[string]bool, error) {
	keys, err := s.Hidden()
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	return set, err
}

// Limits lists the contracts and windows that can be drawn, with whether each is shown. It is empty
// until the first usage arrives.
func (s *Service) Limits() (list []LimitContract, err error) {
	defer func() { err = fault.Boundary(s.Logger, "display.limits", err) }()
	hidden, err := s.hiddenSet()
	if err != nil {
		return nil, err
	}
	stats, _ := s.State.Snapshot()
	return contractsOf(stats, hidden), nil
}

// SetShown saves that the windows with the given keys are shown or hidden and redraws at once. The
// other windows keep their state. When saving fails nothing changes. It returns the list as it is now.
func (s *Service) SetShown(keys []string, shown bool) (list []LimitContract, err error) {
	defer func() { err = fault.Boundary(s.Logger, "display.setShown", err) }()
	if err := s.Show(keys, shown); err != nil {
		return nil, err
	}
	s.State.Touch()
	return s.Limits()
}

// Preview returns the latest TURZX image as a PNG data URL, or "" before the first image.
func (s *Service) Preview() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preview
}

// PreviewPage returns one manually selected window-preview page. The requested page is clamped to
// the currently available range so a provider-count decrease keeps the preview valid.
func (s *Service) PreviewPage(page int) PreviewFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	pages := s.previewPages
	if len(pages) == 0 {
		if s.preview == "" {
			return PreviewFrame{}
		}
		pages = []string{s.preview}
	}
	page = min(max(page, 0), len(pages)-1)
	return PreviewFrame{Image: pages[page], Page: page, PageCount: len(pages)}
}

func previewDataURL(img *image.RGBA) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Run redraws when the state changes and at least as often as the selected profile requires. The
// application redraw interval remains an upper bound so test/server builds can request faster
// refreshes. The selected profile owns the logical size and layout for each image. Each image goes
// to the preview in s and to output. It is a function, not a method, so Wails does not bind it.
func Run(ctx context.Context, s *Service, renderer *Renderer, state *usage.State, redraw time.Duration, profile func() DisplayProfile, style func() Style, output func(*image.RGBA), emit func(string, any), logger *slog.Logger) {
	ticker := time.NewTicker(redraw)
	defer ticker.Stop()
	activeInterval := redraw

	for {
		stats, source := state.Snapshot()
		// A selection that cannot be read draws every window, as style does for Gauges.
		hidden, err := s.hiddenSet()
		if err != nil {
			logger.Warn("hidden_limits_unavailable", "cause", err)
		}
		selected := DefaultProfile()
		if profile != nil {
			selected = profile()
		}
		paging := DefaultCompactPagingSettings()
		if s.Paging != nil {
			paging = s.Paging()
		}
		if interval := selected.redrawInterval(redraw, paging); interval != activeInterval {
			ticker.Reset(interval)
			activeInterval = interval
		}

		visibleStats := withoutHidden(stats, hidden)
		now := time.Now()
		selectedStyle := style()
		img := selected.RenderWithPaging(renderer, visibleStats, now, source, selectedStyle, paging)
		output(img)

		devicePreview, err := previewDataURL(img)
		if err != nil {
			logger.Error("preview_encode_failed", "cause", err)
		} else {
			pages := []string{devicePreview}
			if pager, ok := selected.Layout.(previewPager); ok {
				renderedPages := pager.previewPages(renderer, visibleStats, now, source, selectedStyle)
				encodedPages := make([]string, 0, len(renderedPages))
				for _, pageImage := range renderedPages {
					dataURL, err := previewDataURL(pageImage)
					if err != nil {
						logger.Error("preview_page_encode_failed", "cause", err)
						encodedPages = nil
						break
					}
					encodedPages = append(encodedPages, dataURL)
				}
				if len(encodedPages) > 0 {
					pages = encodedPages
				}
			}
			s.mu.Lock()
			s.preview = devicePreview
			s.previewPages = pages
			s.mu.Unlock()
			emit(Updated, nil)
		}
		select {
		case <-ctx.Done():
			return
		case <-state.Changed():
		case <-ticker.C:
		}
	}
}
