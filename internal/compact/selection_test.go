package compact

import (
	"reflect"
	"testing"
	"time"

	"token-monitor-turzx/internal/usage"
)

func percent(n float64) *float64 { return &n }
func window(kind, label string, remaining *float64) usage.Window {
	return usage.Window{Kind: kind, Label: label, ShowMeter: true, RemainingPercent: remaining}
}
func provider(name string, windows ...usage.Window) usage.Provider {
	return usage.Provider{Provider: name, AccountLabel: "account", PlanLabel: "plan", Windows: windows}
}
func names(stats *usage.Stats) []string {
	var result []string
	for _, p := range stats.Limits.Providers {
		result = append(result, p.Provider)
	}
	return result
}

func TestSelectEmptyAndProviderCounts(t *testing.T) {
	if got := Select(nil, nil); len(got) != 1 || got[0] != nil {
		t.Fatalf("missing stats: %+v", got)
	}
	for _, count := range []int{0, 1, 2, 3, 7} {
		stats := &usage.Stats{Periods: usage.Periods{Today: usage.Period{TotalTokens: 123}}}
		for i := 0; i < count; i++ {
			stats.Limits.Providers = append(stats.Limits.Providers, provider(string(rune('a'+i)), window("daily", "D", percent(float64(i)))))
		}
		pages := Select(stats, nil)
		want := max(1, (count+1)/2)
		if len(pages) != want {
			t.Fatalf("providers %d: pages=%d, want %d", count, len(pages), want)
		}
		var got []string
		for _, page := range pages {
			if len(page.Limits.Providers) > 2 || page.Periods.Today.TotalTokens != 123 {
				t.Fatalf("providers %d: unexpected page %+v", count, page)
			}
			got = append(got, names(page)...)
		}
		var expected []string
		for i := 0; i < count; i++ {
			expected = append(expected, string(rune('a'+i)))
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("providers %d: order %v, want %v", count, got, expected)
		}
	}
}

func TestSelectWindowsKnownRiskThenOuterRingShorter(t *testing.T) {
	p := provider("claude",
		window("unknown", "unreported", nil),
		window("weekly", "weekly", percent(5)),
		window("session", "session", percent(15)),
		window("daily", "daily", percent(50)),
	)
	pages := Select(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{p}}}, nil)
	got := pages[0].Limits.Providers[0].Windows
	if len(got) != 2 || got[0].Label != "session" || got[1].Label != "weekly" {
		t.Fatalf("picked shortest first after taking most depleted two: %+v", got)
	}
	if len(p.Windows) != 4 || p.Windows[0].Label != "unreported" {
		t.Fatal("original window slice was mutated")
	}
}

func TestSelectWindowDurationUnknownAndStableTies(t *testing.T) {
	p := provider("p",
		window("other", "unknown-duration", percent(10)),
		window("session", "session", percent(10)),
		window("daily", "daily", percent(10)),
	)
	pages := Select(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{p}}}, nil)
	got := pages[0].Limits.Providers[0].Windows
	if got[0].Label != "session" || got[1].Label != "unknown-duration" {
		t.Fatalf("unknown duration comes after known in selected first two: %+v", got)
	}
	p = provider("p", window("unknown", "first", nil), window("unknown", "second", nil))
	got = Select(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{p}}}, nil)[0].Limits.Providers[0].Windows
	if got[0].Label != "first" || got[1].Label != "second" {
		t.Fatalf("stable ties: %+v", got)
	}
	specific := window("unknown", "specified", percent(2))
	specific.WindowMinutes = percent(20)
	p = provider("p", window("daily", "fallback", percent(3)), specific)
	got = Select(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{p}}}, nil)[0].Limits.Providers[0].Windows
	if got[0].Label != "specified" {
		t.Fatalf("reported duration must override kind fallback: %+v", got)
	}
}

func TestSelectProviderRiskOrderAndHidden(t *testing.T) {
	stats := &usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{
		provider("unknown", window("session", "S", nil)),
		provider("higher", window("session", "S", percent(50))),
		provider("same1", window("session", "S", percent(20))),
		provider("same2", window("session", "S", percent(20))),
		provider("hidden-low", window("session", "H", percent(0)), window("weekly", "W", percent(60))),
		provider("not-metered", usage.Window{Kind: "daily", ShowMeter: false}),
	}}}
	hidden := []string{windowKey(stats.Limits.Providers[4], stats.Limits.Providers[4].Windows[0])}
	pages := Select(stats, hidden)
	var got []string
	for _, page := range pages {
		got = append(got, names(page)...)
	}
	want := []string{"same1", "same2", "higher", "hidden-low", "unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider priority = %v, want %v", got, want)
	}
	if pages[1].Limits.Providers[1].Windows[0].Label != "W" {
		t.Fatal("hidden lower-risk window remains visible")
	}
}

func TestSelectNoVisibleWindowsPreservesTokens(t *testing.T) {
	p := provider("p", window("session", "S", percent(10)))
	stats := &usage.Stats{Periods: usage.Periods{AllTime: usage.Period{CostUSD: 55}}, Limits: usage.Limits{Providers: []usage.Provider{p}}}
	pages := Select(stats, []string{windowKey(p, p.Windows[0])})
	if len(pages) != 1 || len(pages[0].Limits.Providers) != 0 || pages[0].Periods.AllTime.CostUSD != 55 {
		t.Fatalf("empty visible selection: %+v", pages)
	}
}

func TestSelectDeepCopyIncludingPointers(t *testing.T) {
	reset := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	original := window("session", "S", percent(30))
	original.UsedPercent = percent(70)
	original.WindowMinutes = percent(300)
	original.ResetsAt = &reset
	stats := &usage.Stats{Periods: usage.Periods{Month: usage.Period{TotalTokens: 54321}}, Limits: usage.Limits{Providers: []usage.Provider{provider("p", original)}}}
	copyStats := Select(stats, nil)[0]
	copyWindow := &copyStats.Limits.Providers[0].Windows[0]
	*copyWindow.RemainingPercent = 99
	*copyWindow.UsedPercent = 1
	*copyWindow.WindowMinutes = 1
	*copyWindow.ResetsAt = reset.Add(time.Hour)
	copyWindow.Kind = "altered"
	copyStats.Limits.Providers[0].Provider = "other"
	copyStats.Periods.Month.TotalTokens = 0
	if *stats.Limits.Providers[0].Windows[0].RemainingPercent != 30 ||
		*stats.Limits.Providers[0].Windows[0].UsedPercent != 70 ||
		*stats.Limits.Providers[0].Windows[0].WindowMinutes != 300 ||
		!stats.Limits.Providers[0].Windows[0].ResetsAt.Equal(reset) ||
		stats.Limits.Providers[0].Windows[0].Kind != "session" ||
		stats.Limits.Providers[0].Provider != "p" || stats.Periods.Month.TotalTokens != 54321 {
		t.Fatalf("changed source stats: %+v", stats)
	}
}
