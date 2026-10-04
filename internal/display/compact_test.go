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

func TestCompactProvidersKeepInputOrderAndAtMostTwo(t *testing.T) {
	limits := usage.Limits{Providers: []usage.Provider{
		{Provider: "empty", Windows: []usage.Window{{ShowMeter: false}}},
		{Provider: "first", AccountLabel: "Account", Windows: []usage.Window{{ShowMeter: true}}},
		{Provider: "second", PlanLabel: "Plus", Windows: []usage.Window{{ShowMeter: true}}},
		{Provider: "third", PlanLabel: "Pro", Windows: []usage.Window{{ShowMeter: true}}},
	}}
	providers := compactProviders(limits)
	if len(providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(providers))
	}
	if providers[0].name != "first" || providers[0].plan != "Account" || providers[1].name != "second" || providers[1].plan != "Plus" {
		t.Fatalf("providers = %+v", providers)
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
