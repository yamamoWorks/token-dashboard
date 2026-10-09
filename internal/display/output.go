package display

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"slices"
	"time"

	"token-monitor-turzx/internal/turzx"
)

// Output sends images to one TURZX in order. Only the newest pending image is kept.
type Output struct {
	target  func() (string, error)
	logger  *slog.Logger
	pending chan *image.RGBA
	done    chan struct{}
}

// NewOutput sends to the device ID that target returns at the time of each send.
func NewOutput(target func() (string, error), logger *slog.Logger) *Output {
	return &Output{target: target, logger: logger, pending: make(chan *image.RGBA, 1), done: make(chan struct{})}
}

// Submit replaces any image still waiting to be sent. It must be called from one goroutine.
func (o *Output) Submit(img *image.RGBA) {
	select {
	case <-o.pending:
	default:
	}
	o.pending <- img
}

// Run sends until ctx ends, then performs the device-specific exit and closes it.
func (o *Output) Run(ctx context.Context) {
	defer close(o.done)
	var conn turzx.FrameSender
	var connID, lastFailure string
	var latest *image.RGBA
	retry := false
	retryTicker := time.NewTicker(time.Second)
	defer retryTicker.Stop()
	fail := func(event string, err error) {
		// Log a failure once until it changes, not on every image.
		if err.Error() != lastFailure {
			lastFailure = err.Error()
			o.logger.Warn(event, "cause", err)
		}
	}
	trySend := func(img *image.RGBA) {
		id, err := o.target()
		if err != nil {
			fail("turzx_target_failed", err)
			retry = true
			return
		}
		if id == "" {
			if conn != nil {
				conn.Close()
				conn = nil
			}
			retry = true
			return
		}
		if conn != nil && connID != id {
			conn.Close()
			conn = nil
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
			if conn, err = turzx.OpenFrameSender(id); err != nil {
				conn = nil
				fail("turzx_open_failed", err)
				retry = true
				return
			}
			connID = id
		}
		if err := conn.SendFrame(img); err != nil {
			conn.Close()
			conn = nil
			fail("turzx_send_failed", err)
			retry = true
			return
		}
		retry = false
		lastFailure = ""
	}
	for {
		select {
		case <-ctx.Done():
			if conn != nil {
				if err := conn.Exit(); err != nil {
					o.logger.Warn("turzx_exit_failed", "cause", err)
				}
				conn.Close()
			}
			return
		case img := <-o.pending:
			latest = img
			trySend(latest)
		case <-retryTicker.C:
			gotFrame := false
			select {
			case img := <-o.pending:
				latest = img
				gotFrame = true
			default:
			}
			if latest == nil {
				continue
			}
			if gotFrame {
				trySend(latest)
				continue
			}
			id, targetErr := o.target()
			if targetErr == nil && conn != nil {
				connected, listErr := turzx.List()
				present := listErr == nil && slices.ContainsFunc(connected, func(d turzx.Device) bool { return d.ID == connID })
				if id != connID || !present {
					conn.Close()
					conn = nil
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
