package display

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"slices"
	"sync"
	"time"

	"token-monitor-turzx/internal/turzx"
)

// Output sends images to one TURZX in order. Only the newest pending image is kept.
type Output struct {
	target  func() (string, error)
	logger  *slog.Logger
	pending chan *image.RGBA
	done    chan struct{}

	// Power transitions are queued in order without blocking the Windows window procedure.
	// All device I/O remains owned by Run.
	powerMu      sync.Mutex
	powerDesired bool
	powerQueue   []bool
	powerChanged chan struct{}

	// Injectable at the transport boundary for hardware-free state transition tests.
	open func(string) (turzx.FrameSender, error)
	list func() ([]turzx.Device, error)
}

// NewOutput sends to the device ID that target returns at the time of each send.
func NewOutput(target func() (string, error), logger *slog.Logger) *Output {
	return &Output{
		target: target, logger: logger, pending: make(chan *image.RGBA, 1), done: make(chan struct{}),
		powerDesired: true, powerChanged: make(chan struct{}, 1),
		open: turzx.OpenFrameSender, list: turzx.List,
	}
}

// SetEnabled queues a transition from the Windows session display monitor.
// Duplicate states are ignored; a short OFF->ON sequence is retained in order.
// No USB operation is performed on the caller's thread.
func (o *Output) SetEnabled(enabled bool) {
	o.powerMu.Lock()
	defer o.powerMu.Unlock()
	if o.powerDesired == enabled {
		return
	}
	o.powerDesired = enabled
	o.powerQueue = append(o.powerQueue, enabled)
	select {
	case o.powerChanged <- struct{}{}:
	default:
	}
}

func (o *Output) nextPower() (bool, bool) {
	o.powerMu.Lock()
	defer o.powerMu.Unlock()
	if len(o.powerQueue) == 0 {
		return false, false
	}
	state := o.powerQueue[0]
	o.powerQueue[0] = false
	o.powerQueue = o.powerQueue[1:]
	return state, true
}

func (o *Output) hasPendingPower() bool {
	o.powerMu.Lock()
	defer o.powerMu.Unlock()
	return len(o.powerQueue) != 0
}

// Submit replaces any image still waiting to be sent. It must be called from one goroutine.
func (o *Output) Submit(img *image.RGBA) {
	select {
	case <-o.pending:
	default:
	}
	o.pending <- img
}

// Run owns the device connection and handles power transitions and frames serially.
func (o *Output) Run(ctx context.Context) {
	defer close(o.done)
	var conn turzx.FrameSender
	var connID, lastFailure string
	var latest *image.RGBA
	active, retry := true, false
	retryTicker := time.NewTicker(time.Second)
	defer retryTicker.Stop()

	fail := func(event string, err error) {
		if err.Error() != lastFailure {
			lastFailure = err.Error()
			o.logger.Warn(event, "cause", err)
		}
	}
	closeConn := func(reset bool) {
		if conn == nil {
			return
		}
		if reset {
			if err := conn.Exit(); err != nil {
				o.logger.Warn("turzx_exit_failed", "cause", err)
			}
		}
		if err := conn.Close(); err != nil {
			o.logger.Warn("turzx_close_failed", "cause", err)
		}
		conn = nil
		connID = ""
	}
	drainLatest := func() (gotFrame bool) {
		for {
			select {
			case img := <-o.pending:
				latest = img
				gotFrame = true
			default:
				return gotFrame
			}
		}
	}
	applyPower := func() (resumed bool) {
		for {
			on, ok := o.nextPower()
			if !ok {
				return resumed
			}
			if !on {
				active = false
				retry = false
				closeConn(true)
			} else {
				active = true
				retry = true
				resumed = true
			}
		}
	}
	trySend := func(img *image.RGBA) {
		if !active {
			return
		}
		id, err := o.target()
		if err != nil {
			fail("turzx_target_failed", err)
			retry = true
			return
		}
		if id == "" {
			closeConn(false)
			retry = true
			return
		}
		if conn != nil && connID != id {
			closeConn(false)
		}
		if !frameMatchesDevice(id, img) {
			dimensions := "nil"
			if img != nil {
				dimensions = fmt.Sprintf("%dx%d", img.Bounds().Dx(), img.Bounds().Dy())
			}
			fail("turzx_profile_mismatch", fmt.Errorf("selected TURZX device does not match %s frame; image skipped", dimensions))
			retry = false
			return
		}
		if conn == nil {
			if conn, err = o.open(id); err != nil {
				conn = nil
				fail("turzx_open_failed", err)
				retry = true
				return
			}
			connID = id
		}
		if err := conn.SendFrame(img); err != nil {
			closeConn(false)
			fail("turzx_send_failed", err)
			retry = true
			return
		}
		retry = false
		lastFailure = ""
	}

	for {
		// Always process transitions before starting the next synchronous USB transfer.
		if applyPower() && active {
			drainLatest()
			if latest != nil {
				trySend(latest)
			}
		}
		select {
		case <-ctx.Done():
			closeConn(true)
			return
		case <-o.powerChanged:
			// The ordered transition queue is drained at the top of the loop.
		case img := <-o.pending:
			latest = img
			if !o.hasPendingPower() && active {
				trySend(latest)
			}
		case <-retryTicker.C:
			if o.hasPendingPower() {
				continue
			}
			gotFrame := drainLatest()
			if !active || latest == nil {
				continue
			}
			if gotFrame {
				trySend(latest)
				continue
			}
			id, targetErr := o.target()
			if targetErr == nil && conn != nil {
				connected, listErr := o.list()
				present := listErr == nil && slices.ContainsFunc(connected, func(d turzx.Device) bool { return d.ID == connID })
				if id != connID || !present {
					closeConn(false)
					retry = true
				}
			}
			if targetErr == nil && (conn == nil || id == "") {
				retry = true
			}
			if retry {
				trySend(latest)
			}
		}
	}
}

func frameMatchesDevice(id string, img *image.RGBA) bool {
	if img == nil {
		return false
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if turzx.IsRevA(id) {
		return w == 480 && h == 320
	}
	return w == 1920 && h == 462
}

// Wait blocks until Run has returned.
func (o *Output) Wait() { <-o.done }
