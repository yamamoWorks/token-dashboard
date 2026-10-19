package compact

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"token-monitor-turzx/internal/display"
	"token-monitor-turzx/internal/usage"
)

func bridgeLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func pngURL(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func bridgeJPEG(t *testing.T, width, height int, fill color.Gray) []byte {
	t.Helper()
	src := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBridgeOnlyBlackPortraitJPEGPassesOutput(t *testing.T) {
	black := bridgeJPEG(t, sourceHeight, sourceWidth, color.Gray{})
	nonblack := bridgeJPEG(t, sourceHeight, sourceWidth, color.Gray{Y: 1})
	for _, tc := range []struct {
		name string
		data []byte
		want bool
	}{
		{"away", black, true},
		{"ordinary portrait JPEG", nonblack, false},
		{"landscape black JPEG", bridgeJPEG(t, sourceWidth, sourceHeight, color.Gray{}), false},
		{"wrong size", bridgeJPEG(t, Width, Height, color.Gray{}), false},
		{"PNG on JPEG callback", encodedCanvas(t, color.RGBA{A: 255}), false},
		{"truncated JPEG", black[:len(black)/2], false},
		{"unknown", []byte("GIF89a"), false},
		{"empty", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]byte
			b := NewBridge(context.Background(), nil, func(data []byte) { got = append(got, bytes.Clone(data)) }, bridgeLogger())
			b.OnOutput(tc.data)
			if (len(got) == 1) != tc.want || len(got) > 1 {
				t.Fatalf("submitted %d frames, want accepted=%v", len(got), tc.want)
			}
			if tc.want && !bytes.Equal(got[0], black) {
				t.Fatal("away instruction changed")
			}
		})
	}
}

func TestBridgeValidatesPreviewAndRecovers(t *testing.T) {
	valid := encodedCanvas(t, color.RGBA{B: 255, A: 255})
	var small bytes.Buffer
	if err := png.Encode(&small, image.NewRGBA(image.Rect(0, 0, Width, Height))); err != nil {
		t.Fatal(err)
	}
	for _, preview := range []string{
		"", "not a data URL", "data:image/png;base64,%%%",
		"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(valid),
		pngURL(bridgeJPEG(t, sourceHeight, sourceWidth, color.Gray{})),
		pngURL(small.Bytes()), pngURL(valid[:len(valid)-5]),
	} {
		t.Run(preview[:min(len(preview), 40)], func(t *testing.T) {
			current := preview
			var got [][]byte
			b := NewBridge(context.Background(), func() string { return current }, func(data []byte) { got = append(got, data) }, bridgeLogger())
			b.OnUpdated()
			if len(got) != 0 {
				t.Fatal("invalid preview reached output")
			}
			current = pngURL(valid)
			b.OnUpdated()
			if len(got) != 1 || !bytes.Equal(got[0], valid) {
				t.Fatal("next valid preview did not recover")
			}
		})
	}
	var uninitialized *Bridge
	uninitialized.OnOutput(nil)
	uninitialized.OnUpdated()
	NewBridge(context.Background(), nil, nil, nil).OnUpdated()
}

// Drive the unchanged Go renderer with encoded fixtures instead of a WebView.
// The output callback precedes Preview publication, so only the Updated wrapper
// can retrieve the fresh PNG. Away beats must not resend the stale Preview.
func TestBridgeDisplayRunAwayHeartbeatAndFirstReturnPNG(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	state := usage.NewState()
	s := &display.Service{State: state, Logger: bridgeLogger(), Hidden: func() ([]string, error) { return nil, nil }}
	frames := make(chan []byte, 32)
	requests := make(chan struct{}, 1)
	initialPNGReleased := make(chan struct{})
	b := NewBridge(ctx, s.Preview, func(data []byte) {
		select {
		case frames <- bytes.Clone(data):
		case <-ctx.Done():
		}
		if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			// Keep the initial Updated callback in flight until the test actor has
			// set away. Otherwise the 100ms beat can start a second Canvas request
			// before that actor runs, and the fake renderer never completes it.
			select {
			case <-initialPNGReleased:
			case <-ctx.Done():
			}
		}
	}, bridgeLogger())
	emit := func(name string, _ any) {
		switch name {
		case display.Updated:
			b.OnUpdated()
		case display.RenderRequested:
			select {
			case requests <- struct{}{}:
			case <-ctx.Done():
			}
		}
	}
	r := display.NewCanvasRenderer(s, emit)
	done := make(chan struct{})
	go func() {
		defer close(done)
		display.Run(ctx, s, r, state, 100*time.Millisecond, func() display.Style { return display.Gauges }, b.OnOutput, emit, s.Logger)
	}()
	t.Cleanup(func() { cancel(); waitSignal(t, "display shutdown", done) })
	complete := func(data []byte, failure string) {
		t.Helper()
		waitSignal(t, "render request", requests)
		req := s.RenderRequest()
		if req == nil {
			t.Fatal("missing request")
		}
		if err := s.CompleteFrame(req.ID, base64.StdEncoding.EncodeToString(data), base64.StdEncoding.EncodeToString(bridgeJPEG(t, sourceHeight, sourceWidth, color.Gray{Y: 100})), failure); err != nil {
			t.Fatal(err)
		}
	}
	next := func() []byte {
		t.Helper()
		select {
		case frame := <-frames:
			return frame
		case <-ctx.Done():
			t.Fatal("no bridge frame before timeout")
			return nil
		}
	}
	red := encodedCanvas(t, color.RGBA{R: 255, A: 255})
	complete(red, "")
	if !bytes.Equal(next(), red) {
		t.Fatal("initial output was not the published PNG")
	}
	// Model a caller that is scheduled after the next redraw beat under CI load.
	time.Sleep(200 * time.Millisecond)
	display.SetAway(s, true)
	close(initialPNGReleased)
	for i := 0; i < 3; i++ {
		data := next()
		_, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != "jpeg" {
			t.Fatalf("idle heartbeat was not black JPEG: %s %v", format, err)
		}
		if _, err := frameFromImage(data); err != nil {
			t.Fatal(err)
		}
	}
	if s.Preview() != pngURL(red) {
		t.Fatal("away changed preview")
	}
	select {
	case <-requests:
		t.Fatal("away requested a normal frame")
	default:
	}
	display.SetAway(s, false)
	blue := encodedCanvas(t, color.RGBA{B: 255, A: 255})
	complete(nil, "render failed")
	complete(blue, "")
	for {
		data := next()
		if bytes.Equal(data, blue) {
			break
		}
		if bytes.Equal(data, red) {
			t.Fatal("return replayed stale preview")
		}
		if _, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || format != "jpeg" {
			t.Fatal("unexpected frame while returning")
		}
	}
}

func TestBridgeConcurrentCallbacksAndCancellationDuringPreview(t *testing.T) {
	data := encodedCanvas(t, color.RGBA{G: 255, A: 255})
	ctx, cancel := context.WithCancel(context.Background())
	o := NewOutput(nil, bridgeLogger())
	b := NewBridge(ctx, func() string { return pngURL(data) }, o.Submit, bridgeLogger())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() { b.OnUpdated(); b.OnOutput([]byte("bad image")) })
	}
	wg.Wait()
	if len(o.pending) != 1 || !bytes.Equal(<-o.pending, data) {
		t.Fatal("concurrent events did not retain exactly the latest PNG")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	b.preview = func() string { close(entered); <-release; return pngURL(data) }
	done := make(chan struct{})
	go func() { b.OnUpdated(); close(done) }()
	waitSignal(t, "preview read", entered)
	cancel()
	close(release)
	waitSignal(t, "cancelled callback", done)
	b.OnUpdated()
	b.OnOutput(bridgeJPEG(t, sourceHeight, sourceWidth, color.Gray{}))
	if len(o.pending) != 0 {
		t.Fatal("cancelled bridge queued a frame")
	}
}

func TestBridgeLatestPNGAndShutdownReleaseTransport(t *testing.T) {
	port := &observedPort{firstCommand: make(chan struct{}), releaseFirst: make(chan struct{}), sentFrames: make(chan struct{}, 3)}
	o := NewOutput(func() (string, error) { return RevAID, nil }, bridgeLogger())
	o.open = func(string) (*Conn, error) { return newConn(port) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	red := encodedCanvas(t, color.RGBA{R: 255, A: 255})
	green := encodedCanvas(t, color.RGBA{G: 255, A: 255})
	blue := encodedCanvas(t, color.RGBA{B: 255, A: 255})
	current := pngURL(red)
	b := NewBridge(ctx, func() string { return current }, o.Submit, bridgeLogger())
	go o.Run(ctx)
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(port.releaseFirst) }); cancel(); o.Wait() })
	b.OnUpdated()
	waitSignal(t, "first strip", port.firstCommand)
	current = pngURL(green)
	b.OnUpdated()
	current = pngURL(blue)
	b.OnUpdated()
	release.Do(func() { close(port.releaseFirst) })
	waitSignal(t, "red frame", port.sentFrames)
	waitSignal(t, "latest blue frame", port.sentFrames)
	cancel()
	o.Wait()
	if len(port.writes) != 1+2*60*3+1 {
		t.Fatalf("unexpected writes: %d", len(port.writes))
	}
	at := 16 + 60*(6+5120) + 6
	if !bytes.Equal(port.data[at:at+2], []byte{0x1f, 0}) {
		t.Fatal("last serial frame was not the blue PNG crop")
	}
	if port.closeCount != 1 || !bytes.HasSuffix(port.data, resetCommand()) {
		t.Fatal("shutdown did not RESET and close the transport")
	}
	b.OnUpdated()
	if len(o.pending) != 0 {
		t.Fatal("bridge submitted after shutdown")
	}
}
