package compact

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"token-monitor-turzx/internal/usage"
)

func waitPage(t *testing.T, state *usage.State) (*usage.Stats, string) {
	t.Helper()
	select {
	case <-state.Changed():
		return state.Snapshot()
	case <-time.After(2 * time.Second):
		t.Fatal("no compact update")
		return nil, ""
	}
}

func TestMirrorRotationAndSourceSwitch(t *testing.T) {
	all, compactState := usage.NewState(), usage.NewState()
	all.Set(&usage.Stats{Periods: usage.Periods{Today: usage.Period{TotalTokens: 25}}, Limits: usage.Limits{Providers: []usage.Provider{
		provider("first", window("session", "S", percent(1))),
		provider("second", window("session", "S", percent(2))),
		provider("third", window("session", "S", percent(3))),
	}}})
	turns := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() { defer close(finished); (&Mirror{All: all, Compact: compactState}).run(ctx, turns) }()
	page, source := waitPage(t, compactState)
	if source != "Local" || !equalNames(names(page), []string{"first", "second"}) || page.Periods.Today.TotalTokens != 25 {
		t.Fatalf("first page: %s %+v", source, page)
	}
	turns <- time.Now()
	page, _ = waitPage(t, compactState)
	if !equalNames(names(page), []string{"third"}) {
		t.Fatalf("second page: %+v", page)
	}
	// Frequent polling updates must not restart the rotation at the first page.
	all.Set(&usage.Stats{Periods: usage.Periods{Today: usage.Period{TotalTokens: 26}}, Limits: usage.Limits{Providers: []usage.Provider{
		provider("first", window("session", "S", percent(1))),
		provider("second", window("session", "S", percent(2))),
		provider("third", window("session", "S", percent(3))),
	}}})
	page, _ = waitPage(t, compactState)
	if !equalNames(names(page), []string{"third"}) || page.Periods.Today.TotalTokens != 26 {
		t.Fatalf("refresh lost page or totals: %+v", page)
	}
	turns <- time.Now()
	page, _ = waitPage(t, compactState)
	if !equalNames(names(page), []string{"first", "second"}) {
		t.Fatalf("wrapped page: %+v", page)
	}
	all.SetSource("Hub")
	page, source = waitPage(t, compactState)
	if page != nil || source != "Hub" {
		t.Fatalf("new source must clear old data immediately: %s %+v", source, page)
	}
	all.Set(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{provider("hub", window("daily", "D", percent(80)))}}})
	page, source = waitPage(t, compactState)
	if !equalNames(names(page), []string{"hub"}) || source != "Hub" {
		t.Fatalf("hub data: %s %+v", source, page)
	}
	turns <- time.Now()
	select {
	case <-compactState.Changed():
		t.Fatal("single-page snapshot should never rotate")
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("mirror did not stop")
	}
}

func TestMirrorHiddenTouchAndFallback(t *testing.T) {
	all, compactState := usage.NewState(), usage.NewState()
	p := provider("first", window("session", "S", percent(1)), window("weekly", "W", percent(60)))
	all.Set(&usage.Stats{Limits: usage.Limits{Providers: []usage.Provider{p, provider("second", window("daily", "D", percent(2)))}}})
	var mu sync.Mutex
	var hidden []string
	var readErr error
	getHidden := func() ([]string, error) { mu.Lock(); defer mu.Unlock(); return hidden, readErr }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go (&Mirror{All: all, Compact: compactState, Hidden: getHidden}).run(ctx, make(chan time.Time))
	page, _ := waitPage(t, compactState)
	if page.Limits.Providers[0].Provider != "first" {
		t.Fatalf("before hidden: %+v", page)
	}
	mu.Lock()
	hidden = []string{windowKey(p, p.Windows[0])}
	mu.Unlock()
	all.Touch() // display.Service.SetShown wakes the full state in production.
	page, _ = waitPage(t, compactState)
	if page.Limits.Providers[0].Provider != "second" ||
		page.Limits.Providers[1].Windows[0].Label != "W" {
		t.Fatalf("hidden selection was not immediately applied: %+v", page)
	}
	mu.Lock()
	readErr = errors.New("unavailable")
	mu.Unlock()
	all.Touch()
	page, _ = waitPage(t, compactState)
	if page.Limits.Providers[0].Provider != "first" {
		t.Fatalf("error fallback must show all: %+v", page)
	}
	full, _ := all.Snapshot()
	if len(full.Limits.Providers[0].Windows) != 2 {
		t.Fatal("full UI list was damaged")
	}
}

func equalNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
