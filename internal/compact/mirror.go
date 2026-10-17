package compact

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"token-monitor-turzx/internal/usage"
)

// PageInterval is the fixed compact page rotation interval. It does not read
// the upstream display's user-configurable cycling settings.
const PageInterval = 10 * time.Second

// Mirror watches the complete usage snapshot and publishes only the visible
// two-provider page to Compact. All remains unchanged and serves the settings UI.
type Mirror struct {
	All     *usage.State
	Compact *usage.State
	Hidden  func() ([]string, error)
	Logger  *slog.Logger
}

// Run owns the only compact page timer. The display renderer's built-in
// page cycling must be disabled for the compact state.
func (m *Mirror) Run(ctx context.Context) {
	ticker := time.NewTicker(PageInterval)
	defer ticker.Stop()
	m.run(ctx, ticker.C)
}

func (m *Mirror) run(ctx context.Context, turns <-chan time.Time) {
	page := 0
	var pages []*usage.Stats
	var currentSource string
	var previousHidden []string
	initialized := false

	refresh := func() {
		stats, source := m.All.Snapshot()
		var hidden []string
		if m.Hidden != nil {
			var err error
			hidden, err = m.Hidden()
			if err != nil {
				if m.Logger != nil {
					m.Logger.Warn("compact_hidden_limits_unavailable", "cause", err)
				}
				// Match display.Run's fallback: an unreadable selection hides nothing.
				hidden = nil
			}
		}
		// A source switch, changed selection, or waiting state begins at page one.
		// Normal stats updates preserve the page so frequent polling cannot starve rotation.
		if !initialized || currentSource != source || !slices.Equal(previousHidden, hidden) || stats == nil {
			page = 0
		}
		_, compactSource := m.Compact.Snapshot()
		changedSource := compactSource != source
		if changedSource {
			m.Compact.SetSource(source)
		}
		initialized = true
		currentSource = source
		previousHidden = slices.Clone(hidden)
		pages = Select(stats, hidden)
		if page >= len(pages) {
			page = 0
		}
		if !changedSource || stats != nil {
			m.Compact.Set(pages[page])
		}
	}

	// Initial refresh reads the newest snapshot, including any notification
	// already queued before the mirror started.
	select {
	case <-m.All.Changed():
	default:
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.All.Changed():
			refresh()
		case <-turns:
			if len(pages) > 1 {
				page = (page + 1) % len(pages)
				m.Compact.Set(pages[page])
			}
		}
	}
}
