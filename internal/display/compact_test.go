package display

import (
	"bytes"
	"image"
	"reflect"
	"testing"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"token-monitor-turzx/internal/usage"
)

func value(v float64) *float64 { return &v }

func compactWindowLabels(windows []usage.Window) []string {
	labels := make([]string, len(windows))
	for i, window := range windows {
		labels[i] = window.Label
	}
	return labels
}

func compactProviderNames(providers []compactProviderData) []string {
	names := make([]string, len(providers))
	for i, provider := range providers {
		names[i] = provider.name
	}
	return names
}

func compactProviderFixtures(count int) []compactProviderData {
	providers := make([]compactProviderData, count)
	for i := range providers {
		providers[i] = compactProviderData{name: string(rune('A' + i))}
	}
	return providers
}

func TestCompactGaugeLayoutSize(t *testing.T) {
	if got, want := (CompactGaugeLayout{}).Size(), image.Pt(480, 320); got != want {
		t.Fatalf("compact size = %v, want %v", got, want)
	}
}

func TestCompactWindowSelectionUsesRemainingPriority(t *testing.T) {
	windows := []usage.Window{
		{Label: "unknown", ShowMeter: true},
		{Label: "forty", ShowMeter: true, RemainingPercent: value(40)},
		{Label: "ten-first", ShowMeter: true, RemainingPercent: value(10)},
		{Label: "ten-second", ShowMeter: true, RemainingPercent: value(10)},
		{Label: "hidden", ShowMeter: false, RemainingPercent: value(1)},
	}
	got := compactWindowLabels(compactWindowSelection(windows))
	want := []string{"ten-first", "ten-second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected windows = %v, want %v", got, want)
	}
}

func TestCompactWindowSelectionKeepsUnknownWhenNeeded(t *testing.T) {
	windows := []usage.Window{
		{Label: "unknown-first", ShowMeter: true},
		{Label: "known", ShowMeter: true, RemainingPercent: value(75)},
		{Label: "unknown-second", ShowMeter: true},
	}
	got := compactWindowLabels(compactWindowSelection(windows))
	want := []string{"known", "unknown-first"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected windows = %v, want %v", got, want)
	}
}

func TestCompactRingOrderPutsShorterWindowOutside(t *testing.T) {
	long := 7.0 * 24 * 60
	short := 5.0 * 60
	windows := []usage.Window{
		{Label: "long", WindowMinutes: &long},
		{Label: "short", WindowMinutes: &short},
	}
	got := compactWindowLabels(compactRingOrder(windows))
	want := []string{"short", "long"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ring order = %v, want %v", got, want)
	}
}

func TestCompactProvidersSortByRiskAndKeepStableTies(t *testing.T) {
	limits := usage.Limits{Providers: []usage.Provider{
		{Provider: "unknown-first", AccountLabel: "Account", Windows: []usage.Window{{ShowMeter: true}}},
		{Provider: "forty", PlanLabel: "Plus", Windows: []usage.Window{{ShowMeter: true, RemainingPercent: value(40)}}},
		{Provider: "ten-first", Windows: []usage.Window{{ShowMeter: true, RemainingPercent: value(10)}}},
		{Provider: "ten-second", Windows: []usage.Window{{ShowMeter: true, RemainingPercent: value(10)}, {ShowMeter: false, RemainingPercent: value(1)}}},
		{Provider: "unknown-second", Windows: []usage.Window{{ShowMeter: true}}},
		{Provider: "empty", Windows: []usage.Window{{ShowMeter: false, RemainingPercent: value(0)}}},
	}}
	providers := compactProviders(limits)
	got := compactProviderNames(providers)
	want := []string{"ten-first", "ten-second", "forty", "unknown-first", "unknown-second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("providers = %v, want %v", got, want)
	}
	if providers[2].plan != "Plus" || providers[3].plan != "Account" {
		t.Fatalf("provider plan fallback = %+v", providers)
	}
}

func TestCompactProviderPageCounts(t *testing.T) {
	tests := []struct {
		providers int
		pages     int
		visible   int
	}{
		{providers: 1, pages: 1, visible: 1},
		{providers: 2, pages: 1, visible: 2},
		{providers: 3, pages: 2, visible: 2},
		{providers: 4, pages: 2, visible: 2},
		{providers: 5, pages: 3, visible: 2},
	}
	for _, tt := range tests {
		page, index, pageCount := compactProviderPage(compactProviderFixtures(tt.providers), time.Unix(0, 0))
		if index != 0 || pageCount != tt.pages || len(page) != tt.visible {
			t.Fatalf("providers=%d: index=%d pages=%d visible=%d", tt.providers, index, pageCount, len(page))
		}
	}
}

func TestCompactProviderPageChangesEveryTenSecondsAndVisitsAllProviders(t *testing.T) {
	providers := compactProviderFixtures(5)
	tests := []struct {
		second int64
		page   int
		want   []string
	}{
		{second: 0, page: 0, want: []string{"A", "B"}},
		{second: 9, page: 0, want: []string{"A", "B"}},
		{second: 10, page: 1, want: []string{"C", "D"}},
		{second: 19, page: 1, want: []string{"C", "D"}},
		{second: 20, page: 2, want: []string{"E"}},
		{second: 30, page: 0, want: []string{"A", "B"}},
	}
	for _, tt := range tests {
		page, index, pageCount := compactProviderPage(providers, time.Unix(tt.second, 0))
		if pageCount != 3 || index != tt.page {
			t.Fatalf("second=%d: index=%d pageCount=%d", tt.second, index, pageCount)
		}
		if got := compactProviderNames(page); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("second=%d: providers=%v, want %v", tt.second, got, tt.want)
		}
	}
}

func TestCompactPageIndicatorOnlyForMultiplePages(t *testing.T) {
	if got := compactPageIndicatorText(0, 1); got != "" {
		t.Fatalf("single-page indicator = %q, want empty", got)
	}
	if got := compactPageIndicatorText(0, 2); got != "● ○" {
		t.Fatalf("first indicator = %q", got)
	}
	if got := compactPageIndicatorText(1, 2); got != "○ ●" {
		t.Fatalf("second indicator = %q", got)
	}
	if got := compactPageIndicatorText(2, 7); got != "3 / 7" {
		t.Fatalf("fallback indicator = %q", got)
	}
}

func newCompactTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	medium, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse regular font: %v", err)
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		t.Fatalf("parse bold font: %v", err)
	}
	return &Renderer{
		medium: medium,
		bold:   bold,
		faces:  make(map[faceKey]font.Face),
		icons:  map[string]image.Image{},
		layout: CompactGaugeLayout{},
	}
}

func TestCompactRenderIs480x320AndIgnoresMonthAndAllTime(t *testing.T) {
	now := time.Date(2026, time.October, 5, 1, 0, 0, 0, time.UTC)
	resetSoon := now.Add(2*time.Hour + 13*time.Minute)
	resetWeek := now.Add(3*24*time.Hour + 4*time.Hour)
	fiveHours := 5.0 * 60
	sevenDays := 7.0 * 24 * 60
	stats := &usage.Stats{
		Periods: usage.Periods{
			Today:   usage.Period{TotalTokens: 1240000, CostUSD: 12.48},
			Month:   usage.Period{TotalTokens: 999, CostUSD: 99},
			AllTime: usage.Period{TotalTokens: 9999, CostUSD: 999},
		},
		Limits: usage.Limits{Providers: []usage.Provider{
			{
				Provider:  "Provider With A Very Long Name That Must Be Truncated",
				PlanLabel: "A very long plan label that must stay inside the compact pane",
				Windows: []usage.Window{
					{Kind: "weekly", Label: "Weekly", ShowMeter: true, RemainingPercent: value(74), ResetsAt: &resetWeek, WindowMinutes: &sevenDays},
					{Kind: "session", Label: "5-hour", ShowMeter: true, RemainingPercent: value(62), ResetsAt: &resetSoon, WindowMinutes: &fiveHours},
					{Kind: "billing", Label: "Monthly", ShowMeter: true},
				},
			},
			{
				Provider:     "Second",
				AccountLabel: "Account",
				Windows:      []usage.Window{{Kind: "daily", Label: "Daily", ShowMeter: true}},
			},
		}},
	}

	renderer := newCompactTestRenderer(t)
	img := renderer.Render(stats, now, "Hub", Gauges)
	if got, want := img.Bounds().Size(), image.Pt(480, 320); got != want {
		t.Fatalf("rendered size = %v, want %v", got, want)
	}

	changedPeriods := *stats
	changedPeriods.Periods.Month = usage.Period{TotalTokens: 123456789, CostUSD: 12345.67}
	changedPeriods.Periods.AllTime = usage.Period{TotalTokens: 987654321, CostUSD: 76543.21}
	imgChanged := renderer.Render(&changedPeriods, now, "Hub", Gauges)
	if !bytes.Equal(img.Pix, imgChanged.Pix) {
		t.Fatal("compact render changed when only Month / All Time changed")
	}
}

func TestCompactSingleProviderDoesNotExpandIntoSecondPane(t *testing.T) {
	renderer := newCompactTestRenderer(t)
	stats := &usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{
		{Provider: "Only", Windows: []usage.Window{{Kind: "session", ShowMeter: true, RemainingPercent: value(50)}}},
	}}}
	img := renderer.Render(stats, time.Unix(0, 0), "Hub", Gauges)
	if got := img.RGBAAt(360, 100); got != background {
		t.Fatalf("second pane pixel = %v, want background %v", got, background)
	}
}

func TestCompactRenderChangesAtPageBoundary(t *testing.T) {
	renderer := newCompactTestRenderer(t)
	stats := &usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{
		{Provider: "First", Windows: []usage.Window{{Kind: "session", Label: "First", ShowMeter: true, RemainingPercent: value(10)}}},
		{Provider: "Second", Windows: []usage.Window{{Kind: "session", Label: "Second", ShowMeter: true, RemainingPercent: value(20)}}},
		{Provider: "Third", Windows: []usage.Window{{Kind: "session", Label: "Third", ShowMeter: true, RemainingPercent: value(30)}}},
	}}}
	firstPage := renderer.Render(stats, time.Unix(0, 0), "Hub", Gauges)
	secondPage := renderer.Render(stats, time.Unix(10, 0), "Hub", Gauges)
	if bytes.Equal(firstPage.Pix, secondPage.Pix) {
		t.Fatal("compact render did not change at the 10-second page boundary")
	}
}
