package display

import (
	"context"
	"image"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"token-monitor-turzx/internal/turzx"
)

type outputSpy struct {
	mu sync.Mutex
	events []string
	frames []*image.RGBA
	changed chan struct{}
}

func newOutputSpy() *outputSpy { return &outputSpy{changed: make(chan struct{}, 1)} }

func (s *outputSpy) add(event string, img *image.RGBA) {
	s.mu.Lock()
	s.events = append(s.events, event)
	if img != nil {
		s.frames = append(s.frames, img)
	}
	s.mu.Unlock()
	select { case s.changed <- struct{}{}: default: }
}

func (s *outputSpy) snapshot() ([]string, []*image.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...), append([]*image.RGBA(nil), s.frames...)
}

func (s *outputSpy) await(t *testing.T, want func([]string, []*image.RGBA) bool) {
	t.Helper()
	timeout := time.NewTimer(3*time.Second)
	defer timeout.Stop()
	for {
		events, frames := s.snapshot()
		if want(events, frames) { return }
		select {
		case <-s.changed:
		case <-timeout.C:
			t.Fatalf("timed out waiting for TURZX events; got %v", events)
		}
	}
}

type fakeOutputSender struct { spy *outputSpy }
func (s *fakeOutputSender) SendFrame(img *image.RGBA) error { s.spy.add("send", img); return nil }
func (s *fakeOutputSender) Exit() error { s.spy.add("exit", nil); return nil }
func (s *fakeOutputSender) Close() error { s.spy.add("close", nil); return nil }

func countEvent(events []string, name string) (count int) {
	for _, event := range events { if event == name { count++ } }
	return count
}

func TestOutputPausesOnDisplayOffAndResendsLatestOnOn(t *testing.T) {
	spy := newOutputSpy()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	o := NewOutput(func() (string, error) { return turzx.RevAID, nil }, logger)
	o.open = func(string) (turzx.FrameSender, error) {
		spy.add("open", nil)
		return &fakeOutputSender{spy: spy}, nil
	}
	o.list = func() ([]turzx.Device, error) {
		return []turzx.Device{{ID: turzx.RevAID}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { o.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	first := image.NewRGBA(image.Rect(0, 0, 480, 320))
	second := image.NewRGBA(image.Rect(0, 0, 480, 320))
	o.Submit(first)
	spy.await(t, func(e []string, _ []*image.RGBA) bool { return countEvent(e, "send") == 1 })

	o.SetEnabled(false)
	o.SetEnabled(false) // duplicate notification must not reset twice
	spy.await(t, func(e []string, _ []*image.RGBA) bool {
		return countEvent(e, "exit") == 1 && countEvent(e, "close") == 1
	})
	o.Submit(second)
	// The 1-second retry ticker must not send or reconnect while OFF.
	time.Sleep(1200 * time.Millisecond)
	events, _ := spy.snapshot()
	if countEvent(events, "send") != 1 || countEvent(events, "open") != 1 {
		t.Fatalf("display OFF performed USB I/O: %v", events)
	}

	o.SetEnabled(true)
	o.SetEnabled(true)
	spy.await(t, func(e []string, f []*image.RGBA) bool {
		return countEvent(e, "open") == 2 && countEvent(e, "send") == 2 && len(f) == 2 && f[1] == second
	})
	events, _ = spy.snapshot()
	if countEvent(events, "exit") != 1 {
		t.Fatalf("unexpected reset count: %v", events)
	}
}

func TestOutputKeepsRapidOffOnTransitionsInOrder(t *testing.T) {
	spy := newOutputSpy()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	o := NewOutput(func() (string, error) { return turzx.RevAID, nil }, logger)
	o.open = func(string) (turzx.FrameSender, error) {
		spy.add("open", nil)
		return &fakeOutputSender{spy: spy}, nil
	}
	o.list = func() ([]turzx.Device, error) { return []turzx.Device{{ID: turzx.RevAID}}, nil }
	ctx, cancel := context.WithCancel(context.Background())
	go o.Run(ctx)
	defer func() { cancel(); o.Wait() }()
	o.Submit(image.NewRGBA(image.Rect(0, 0, 480, 320)))
	spy.await(t, func(e []string, _ []*image.RGBA) bool { return countEvent(e, "send") == 1 })
	o.SetEnabled(false)
	o.SetEnabled(true)
	spy.await(t, func(e []string, _ []*image.RGBA) bool {
		return countEvent(e, "exit") == 1 && countEvent(e, "open") == 2 && countEvent(e, "send") == 2
	})
	events, _ := spy.snapshot()
	want := []string{"open", "send", "exit", "close", "open", "send"}
	if len(events) < len(want) {
		t.Fatalf("events %v, want prefix %v", events, want)
	}
	got := events[:len(want)]
	for i := range want {
		if got[i] != want[i] { t.Fatalf("events %v, want prefix %v", events, want) }
	}
}

func TestOutputStartsPausedWhenPowerOffArrivesBeforeRun(t *testing.T) {
	spy := newOutputSpy()
	o := NewOutput(func() (string, error) { return turzx.RevAID, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	o.open = func(string) (turzx.FrameSender, error) {
		spy.add("open", nil)
		return &fakeOutputSender{spy: spy}, nil
	}
	o.SetEnabled(false)
	ctx, cancel := context.WithCancel(context.Background())
	go o.Run(ctx)
	defer func() { cancel(); o.Wait() }()
	o.Submit(image.NewRGBA(image.Rect(0, 0, 480, 320)))
	time.Sleep(150 * time.Millisecond)
	events, _ := spy.snapshot()
	if len(events) != 0 { t.Fatalf("unexpected device access while initially OFF: %v", events) }
	o.SetEnabled(true)
	spy.await(t, func(e []string, _ []*image.RGBA) bool { return countEvent(e, "send") == 1 })
}
