package display

import (
	"cmp"
	"fmt"
	"image"
	"image/draw"
	"math"
	"slices"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"

	"token-monitor-turzx/internal/usage"
)

// CompactGaugeLayout renders a gauge-only dashboard for a 480x320 display.
// It intentionally owns all compact geometry instead of reusing the ultra-wide constants.
type CompactGaugeLayout struct{}

const (
	compactWidth  = 480
	compactHeight = 320

	compactMargin           = 8
	compactPaneGap          = 8
	compactPanelTop         = 48
	compactPanelPad         = 10
	compactPanelBottom      = 8
	compactPagedPanelBottom = 26
	compactPaneWidth        = (compactWidth - 2*compactMargin - compactPaneGap) / 2

	compactGaugeRadius      = 63.0
	compactGaugeInnerRadius = 47.0
	compactGaugeStroke      = 9.0
	compactGaugeCenterY     = 160.0

	compactProvidersPerPage   = 2
	compactPageDuration        = 10 * time.Second
	compactIndicatorBaseline  = 314
	compactIndicatorMaxDots   = 6
)

func (CompactGaugeLayout) Size() image.Point {
	return image.Pt(compactWidth, compactHeight)
}

func (l CompactGaugeLayout) Render(renderer *Renderer, stats *usage.Stats, now time.Time, source string, _ Style) *image.RGBA {
	size := l.Size()
	img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	if stats == nil {
		face := renderer.face(false, 28)
		label := "Waiting for Hub"
		if source == "Local" {
			label = "Waiting for local usage"
		}
		renderer.text(img, face, dim, (size.X-measure(face, label))/2, size.Y/2+10, label)
		return img
	}

	renderer.compactTokenStrip(img, stats.Periods.Today)
	providers, page, pageCount := compactProviderPage(compactProviders(stats.Limits), now)
	for i, provider := range providers {
		x := compactMargin + i*(compactPaneWidth+compactPaneGap)
		renderer.compactProvider(img, provider, x, now, pageCount > 1)
	}
	if pageCount > 1 {
		renderer.compactPageIndicator(img, page, pageCount)
	}
	return img
}

type compactProviderData struct {
	provider string
	name     string
	plan     string
	windows  []usage.Window
}

// compactProviders returns every provider with metered windows ordered by risk. The smallest known
// remaining percentage in the displayed windows comes first, unknown providers follow known ones,
// and stable sorting preserves Hub input order for ties.
func compactProviders(limits usage.Limits) []compactProviderData {
	providers := make([]compactProviderData, 0, len(limits.Providers))
	for _, provider := range limits.Providers {
		windows := compactWindowSelection(provider.Windows)
		if len(windows) == 0 {
			continue
		}
		plan := provider.PlanLabel
		if plan == "" {
			plan = provider.AccountLabel
		}
		providers = append(providers, compactProviderData{
			provider: provider.Provider,
			name:     provider.Provider,
			plan:     plan,
			windows:  compactRingOrder(windows),
		})
	}
	slices.SortStableFunc(providers, func(a, b compactProviderData) int {
		aRisk, aKnown := compactProviderRisk(a)
		bRisk, bKnown := compactProviderRisk(b)
		switch {
		case aKnown && !bKnown:
			return -1
		case !aKnown && bKnown:
			return 1
		case !aKnown:
			return 0
		default:
			return cmp.Compare(aRisk, bRisk)
		}
	})
	return providers
}

func compactProviderRisk(provider compactProviderData) (float64, bool) {
	lowest := math.Inf(1)
	for _, window := range provider.windows {
		if window.RemainingPercent != nil {
			lowest = min(lowest, *window.RemainingPercent)
		}
	}
	return lowest, !math.IsInf(lowest, 1)
}

// compactProviderPage derives the current page solely from now so the existing redraw loop drives
// paging without an additional timer or goroutine.
func compactProviderPage(providers []compactProviderData, now time.Time) ([]compactProviderData, int, int) {
	pageCount := (len(providers) + compactProvidersPerPage - 1) / compactProvidersPerPage
	if pageCount <= 1 {
		return providers, 0, pageCount
	}

	page := int((now.Unix() / int64(compactPageDuration/time.Second)) % int64(pageCount))
	if page < 0 {
		page += pageCount
	}
	start := page * compactProvidersPerPage
	end := min(start+compactProvidersPerPage, len(providers))
	return providers[start:end], page, pageCount
}

func compactPageIndicatorText(page, pageCount int) string {
	if pageCount <= 1 {
		return ""
	}
	if pageCount > compactIndicatorMaxDots {
		return fmt.Sprintf("%d / %d", page+1, pageCount)
	}

	var indicator strings.Builder
	for i := range pageCount {
		if i > 0 {
			indicator.WriteByte(' ')
		}
		if i == page {
			indicator.WriteRune('●')
		} else {
			indicator.WriteRune('○')
		}
	}
	return indicator.String()
}

// compactWindowSelection returns at most two metered windows. Windows with a known remaining
// percentage come first, then lower remaining percentages, with input order preserved for ties.
func compactWindowSelection(windows []usage.Window) []usage.Window {
	metered := make([]usage.Window, 0, len(windows))
	for _, window := range windows {
		if window.ShowMeter {
			metered = append(metered, window)
		}
	}
	slices.SortStableFunc(metered, func(a, b usage.Window) int {
		switch {
		case a.RemainingPercent != nil && b.RemainingPercent == nil:
			return -1
		case a.RemainingPercent == nil && b.RemainingPercent != nil:
			return 1
		case a.RemainingPercent == nil:
			return 0
		default:
			return cmp.Compare(*a.RemainingPercent, *b.RemainingPercent)
		}
	})
	return slices.Clone(metered[:min(2, len(metered))])
}

// compactRingOrder puts the shorter selected window on the outside. Unknown lengths follow known
// lengths, and ties preserve the selection order.
func compactRingOrder(windows []usage.Window) []usage.Window {
	ordered := slices.Clone(windows)
	slices.SortStableFunc(ordered, func(a, b usage.Window) int {
		la, oka := lengthOf(a)
		lb, okb := lengthOf(b)
		switch {
		case oka && !okb:
			return -1
		case !oka && okb:
			return 1
		case !oka:
			return 0
		default:
			return cmp.Compare(la, lb)
		}
	})
	return ordered
}

func (r *Renderer) compactTokenStrip(img *image.RGBA, today usage.Period) {
	labelFace := r.face(false, 15)
	costFace := r.face(false, 18)
	cost := usd(today.CostUSD)
	costX := compactWidth - compactMargin - measure(costFace, cost)
	r.text(img, labelFace, dim, compactMargin+2, 29, "Today")

	x := compactMargin + 2 + measure(labelFace, "Today") + 10
	tokens := commas(fmt.Sprint(today.TotalTokens)) + " tokens"
	size := 22.0
	for size > 14 && measure(r.face(true, size), tokens) > costX-x-12 {
		size -= 2
	}
	r.text(img, r.face(true, size), text, x, 30, tokens)
	r.text(img, costFace, accent, costX, 29, cost)
	fill(img, image.Rect(compactMargin, 40, compactWidth-compactMargin, 41), divider)
}

func (r *Renderer) compactProvider(img *image.RGBA, provider compactProviderData, x int, now time.Time, paged bool) {
	panelBottom := compactPanelBottom
	if paged {
		panelBottom = compactPagedPanelBottom
	}
	rect := image.Rect(x, compactPanelTop, x+compactPaneWidth, compactHeight-panelBottom)
	roundRect(img, rect, 12, divider)
	roundRect(img, rect.Inset(1), 11, panelFill)

	nameFace := r.face(true, 20)
	planFace := r.face(false, 15)
	hx := x + compactPanelPad
	nameRight := x + compactPaneWidth - compactPanelPad
	if icon, ok := r.icons[strings.ToLower(provider.provider)]; ok {
		dst := image.Rect(hx, compactPanelTop+9, hx+20, compactPanelTop+29)
		xdraw.CatmullRom.Scale(img, dst, icon, icon.Bounds(), xdraw.Over, nil)
		hx += 28
	}
	r.text(img, nameFace, text, hx, compactPanelTop+26, truncate(nameFace, provider.name, nameRight-hx))
	if provider.plan != "" {
		r.text(img, planFace, dim, x+compactPanelPad, compactPanelTop+44, truncate(planFace, provider.plan, compactPaneWidth-2*compactPanelPad))
	}

	cx := float64(x + compactPaneWidth/2)
	for i, window := range provider.windows {
		radius := compactGaugeRadius
		if i == 1 {
			radius = compactGaugeInnerRadius
		}
		fillPoly(img, track, arcPoly(cx, compactGaugeCenterY, radius, compactGaugeStroke, 135, 270))
		if window.RemainingPercent != nil {
			value := min(max(*window.RemainingPercent, 0), 100)
			fillPoly(img, gaugeColor(window, now), arcPoly(cx, compactGaugeCenterY, radius, compactGaugeStroke, 135, 270*value/100))
		}
	}

	percentFace := r.face(true, 25)
	windowFace := r.face(false, 15)
	baselines := []int{167}
	if len(provider.windows) == 2 {
		baselines = []int{154, 181}
	}
	for i, window := range provider.windows {
		percent := "—"
		if window.RemainingPercent != nil {
			percent = fmt.Sprintf("%.0f%%", math.Round(*window.RemainingPercent))
		}
		label := windowLabel(window)
		total := measure(percentFace, percent) + 5 + measure(windowFace, label)
		tx := int(cx) - total/2
		r.text(img, percentFace, text, tx, baselines[i], percent)
		r.text(img, windowFace, dim, tx+measure(percentFace, percent)+5, baselines[i], label)
	}

	resetLabelFace := r.face(true, 15)
	resetFace := r.face(false, 14)
	resetY := 253
	for i, window := range provider.windows {
		label := windowLabel(window)
		reset := resetIn(window, now)
		gap := 8
		iconDiameter := 12
		total := measure(resetLabelFace, label) + gap + iconDiameter + 5 + measure(resetFace, reset)
		tx := int(cx) - total/2
		y := resetY + i*25
		r.text(img, resetLabelFace, dim, tx, y, label)
		clock(img, float64(tx+measure(resetLabelFace, label)+gap+iconDiameter/2), float64(y)-5, float64(iconDiameter)/2)
		r.text(img, resetFace, text, tx+measure(resetLabelFace, label)+gap+iconDiameter+5, y, reset)
	}
}

func (r *Renderer) compactPageIndicator(img *image.RGBA, page, pageCount int) {
	indicator := compactPageIndicatorText(page, pageCount)
	if indicator == "" {
		return
	}
	face := r.face(false, 13)
	r.text(img, face, dim, (compactWidth-measure(face, indicator))/2, compactIndicatorBaseline, indicator)
}
