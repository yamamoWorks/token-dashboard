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

	compactMargin      = 8
	compactPaneGap     = 8
	compactPanelTop    = 48
	compactPanelPad    = 10
	compactPanelBottom = 8
	compactPaneWidth   = (compactWidth - 2*compactMargin - compactPaneGap) / 2

	compactGaugeRadius      = 63.0
	compactGaugeInnerRadius = 47.0
	compactGaugeStroke      = 9.0
	compactGaugeCenterY     = 160.0
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
	for i, provider := range compactProviders(stats.Limits) {
		x := compactMargin + i*(compactPaneWidth+compactPaneGap)
		renderer.compactProvider(img, provider, x, now)
	}
	return img
}

type compactProviderData struct {
	provider string
	name     string
	plan     string
	windows  []usage.Window
}

// compactProviders keeps Hub input order and at most the first two providers with metered windows.
// Risk-based provider ordering and paging are deliberately left to the paging layer.
func compactProviders(limits usage.Limits) []compactProviderData {
	providers := make([]compactProviderData, 0, 2)
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
		if len(providers) == 2 {
			break
		}
	}
	return providers
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

func (r *Renderer) compactProvider(img *image.RGBA, provider compactProviderData, x int, now time.Time) {
	rect := image.Rect(x, compactPanelTop, x+compactPaneWidth, compactHeight-compactPanelBottom)
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
