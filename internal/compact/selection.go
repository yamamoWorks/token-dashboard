// Package compact prepares independent, two-provider pages for the 3.5-inch theme.
// The full usage.State remains the source of truth for the settings UI.
package compact

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"token-monitor-turzx/internal/usage"
)

const providersPerPage = 2

// Select prepares immutable, independently owned pages from the latest usage snapshot.
// Selection prefers known, low remaining percentages; display order within a provider
// then prefers shorter windows (the outside ring). Neither the source arrays nor any
// pointer-valued window fields are shared with the returned pages.
func Select(stats *usage.Stats, hiddenKeys []string) []*usage.Stats {
	if stats == nil {
		return []*usage.Stats{nil}
	}
	hidden := make(map[string]bool, len(hiddenKeys))
	for _, key := range hiddenKeys {
		hidden[key] = true
	}
	type rankedProvider struct {
		provider usage.Provider
		risk     float64
	}
	var selected []rankedProvider
	for _, provider := range stats.Limits.Providers {
		windows := make([]usage.Window, 0, len(provider.Windows))
		for _, window := range provider.Windows {
			if !window.ShowMeter || hidden[windowKey(provider, window)] {
				continue
			}
			windows = append(windows, copyWindow(window))
		}
		if len(windows) == 0 {
			continue
		}
		slices.SortStableFunc(windows, func(a, b usage.Window) int {
			return cmp.Compare(remaining(a), remaining(b))
		})
		if len(windows) > 2 {
			windows = windows[:2]
		}
		risk := remaining(windows[0])
		slices.SortStableFunc(windows, func(a, b usage.Window) int {
			return cmp.Compare(duration(a), duration(b))
		})
		selected = append(selected, rankedProvider{
			provider: usage.Provider{
				Provider: provider.Provider, AccountLabel: provider.AccountLabel,
				PlanLabel: provider.PlanLabel, Windows: windows,
			},
			risk: risk,
		})
	}
	slices.SortStableFunc(selected, func(a, b rankedProvider) int { return cmp.Compare(a.risk, b.risk) })
	pages := make([]*usage.Stats, 0, max(1, (len(selected)+providersPerPage-1)/providersPerPage))
	for i := 0; i < len(selected); i += providersPerPage {
		page := &usage.Stats{Periods: stats.Periods}
		for _, item := range selected[i:min(i+providersPerPage, len(selected))] {
			page.Limits.Providers = append(page.Limits.Providers, item.provider)
		}
		pages = append(pages, page)
	}
	if len(pages) == 0 {
		pages = append(pages, &usage.Stats{Periods: stats.Periods})
	}
	return pages
}

// windowKey must match display.windowKey, including the U+001F separator.
func windowKey(provider usage.Provider, window usage.Window) string {
	return strings.Join([]string{provider.Provider, provider.AccountLabel, window.Kind, window.Label}, "\x1f")
}

func remaining(window usage.Window) float64 {
	if window.RemainingPercent == nil || math.IsNaN(*window.RemainingPercent) {
		return math.Inf(1)
	}
	return *window.RemainingPercent
}

var kindMinutes = map[string]float64{
	"session": 300, "daily": 1440, "weekly": 10080, "billing": 43200,
}

func duration(window usage.Window) float64 {
	if window.WindowMinutes != nil && !math.IsNaN(*window.WindowMinutes) {
		return *window.WindowMinutes
	}
	if n, ok := kindMinutes[window.Kind]; ok {
		return n
	}
	return math.Inf(1)
}

func copyWindow(window usage.Window) usage.Window {
	clone := window
	if window.RemainingPercent != nil {
		value := *window.RemainingPercent
		clone.RemainingPercent = &value
	}
	if window.UsedPercent != nil {
		value := *window.UsedPercent
		clone.UsedPercent = &value
	}
	if window.ResetsAt != nil {
		value := *window.ResetsAt
		clone.ResetsAt = &value
	}
	if window.WindowMinutes != nil {
		value := *window.WindowMinutes
		clone.WindowMinutes = &value
	}
	return clone
}
