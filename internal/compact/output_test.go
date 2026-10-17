package compact

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func waitSignal(t *testing.T, name string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

type observedPort struct {
	fakePort
	firstCommand chan struct{}
	releaseFirst chan struct{}
	sentFrames   chan struct{}
	resetStarted chan struct{}
	releaseReset chan struct{}
	chunks       int
	blockOnce    sync.Once
	resetOnce    sync.Once
}

func (p *observedPort) Write(data []byte) (int, error) {
	if len(data) == 6 && data[5] == 197 && p.firstCommand != nil {
		p.blockOnce.Do(func() {
			close(p.firstCommand)
			<-p.releaseFirst
		})
	}
	if bytes.Equal(data, resetCommand()) && p.resetStarted != nil {
		p.resetOnce.Do(func() {
			close(p.resetStarted)
			<-p.releaseReset
		})
	}
	n, err := p.fakePort.Write(data)
	if len(data) == 1280 && n == 1280 {
		p.chunks++
		if p.chunks%60 == 0 && p.sentFrames != nil {
			p.sentFrames <- struct{}{}
		}
	}
	return n, err
}

func TestOutputLatestPendingWhileTransferBusy(t *testing.T) {
	port := &observedPort{
		firstCommand: make(chan struct{}), releaseFirst: make(chan struct{}),
		sentFrames: make(chan struct{}, 3),
	}
	o := NewOutput(func() (string, error) { return RevAID, nil }, nil)
	o.open = func(string) (*Conn, error) { return newConn(port) }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go o.Run(ctx)
	o.Submit(encodedCanvas(t, color.RGBA{R: 255, A: 255}))
	waitSignal(t, "first serial strip", port.firstCommand)
	o.Submit(encodedCanvas(t, color.RGBA{G: 255, A: 255}))
	o.Submit(encodedCanvas(t, color.RGBA{B: 255, A: 255}))
	close(port.releaseFirst)
	waitSignal(t, "first full frame", port.sentFrames)
	waitSignal(t, "second full frame", port.sentFrames)
	cancel()
	o.Wait()
	if want := 1 + 2*60*3 + 1; len(port.writes) != want {
		t.Fatalf("write count %d, want %d (intermediate frame was not dropped)", len(port.writes), want)
	}
	at := func(frameIndex int) int { return 16 + frameIndex*60*(6+5120) + 6 }
	if got := port.data[at(0):at(0)+2]; !bytes.Equal(got, []byte{0, 0xf8}) {
		t.Fatalf("first frame should be red: % x", got)
	}
	if got := port.data[at(1):at(1)+2]; !bytes.Equal(got, []byte{0x1f, 0}) {
		t.Fatalf("next frame should be latest blue, not green: % x", got)
	}
	if port.closeCount != 1 || !bytes.HasSuffix(port.data, resetCommand()) {
		t.Fatalf("shutdown: close=%d, trailing bytes=% x", port.closeCount, port.data[len(port.data)-6:])
	}
}

type closingPort struct {
	*fakePort
	closed chan struct{}
	once sync.Once
}
func (p *closingPort) Close() error {
	err := p.fakePort.Close()
	p.once.Do(func() { close(p.closed) })
	return err
}

func TestOutputRecoversAfterSerialFailureAndReconnects(t *testing.T) {
	failed := &closingPort{fakePort: &fakePort{failAt: 2, failErr: io.ErrClosedPipe}, closed: make(chan struct{})}
	recovered := &observedPort{sentFrames: make(chan struct{}, 1)}
	o := NewOutput(func() (string, error) { return RevAID, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	opens := 0
	o.open = func(string) (*Conn, error) {
		opens++
		if opens == 1 {
			return newConn(failed)
		}
		return newConn(recovered)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go o.Run(ctx)
	pngData := encodedCanvas(t, color.RGBA{R: 255, A: 255})
	o.Submit(pngData)
	waitSignal(t, "failed serial port close", failed.closed)
	// The failed frame is discarded. The next frame must reopen the device.
	o.Submit(pngData)
	waitSignal(t, "frame after reconnect", recovered.sentFrames)
	cancel()
	o.Wait()
	if opens != 2 || failed.closeCount != 1 || recovered.closeCount != 1 {
		t.Fatalf("reconnect opens=%d, failed closed=%d, recovered closed=%d", opens, failed.closeCount, recovered.closeCount)
	}
	if !bytes.HasSuffix(recovered.data, resetCommand()) {
		t.Fatal("reconnected port was not reset on shutdown")
	}
}

func TestOutputResetCloseFinishesBeforeWait(t *testing.T) {
	port := &observedPort{
		sentFrames: make(chan struct{}, 1),
		resetStarted: make(chan struct{}), releaseReset: make(chan struct{}),
	}
	o := NewOutput(func() (string, error) { return RevAID, nil }, nil)
	o.open = func(string) (*Conn, error) { return newConn(port) }
	ctx, cancel := context.WithCancel(context.Background())
	go o.Run(ctx)
	o.Submit(encodedCanvas(t, color.RGBA{A: 255}))
	waitSignal(t, "frame transfer", port.sentFrames)
	cancel()
	waitSignal(t, "RESET command", port.resetStarted)
	done := make(chan struct{})
	go func() { o.Wait(); close(done) }()
	select {
	case <-done:
		t.Fatal("Wait returned before RESET and Close")
	default:
	}
	close(port.releaseReset)
	waitSignal(t, "Wait", done)
	if port.closeCount != 1 || !bytes.HasSuffix(port.data, resetCommand()) {
		t.Fatalf("shutdown did not RESET then Close: count=%d", port.closeCount)
	}
}

func TestOutputInvalidAndFailedOpenAreSafe(t *testing.T) {
	o := NewOutput(func() (string, error) { return RevAID, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var opens int
	openAttempt := make(chan struct{}, 1)
	o.open = func(string) (*Conn, error) {
		opens++
		openAttempt <- struct{}{}
		return nil, errors.New("unplugged")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go o.Run(ctx)
	o.Submit([]byte("not an image"))
	// A later valid frame causes exactly one open attempt; malformed input
	// never reaches the device. Keep only the latest pending data.
	o.Submit(encodedCanvas(t, color.RGBA{A: 255}))
	waitSignal(t, "open error", openAttempt)
	cancel()
	o.Wait()
	if opens != 1 {
		t.Fatalf("open attempts = %d, want 1", opens)
	}
}

func TestSubmitCopiesBufferAndKeepsNewest(t *testing.T) {
	o := NewOutput(nil, nil)
	a := []byte{1, 2, 3}
	o.Submit(a)
	a[0] = 9
	if got := <-o.pending; !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("Submit retained caller buffer: %v", got)
	}
	o.Submit([]byte{4})
	o.Submit([]byte{5})
	if got := <-o.pending; !bytes.Equal(got, []byte{5}) {
		t.Fatalf("pending is not latest: %v", got)
	}
}
