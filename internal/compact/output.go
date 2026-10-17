package compact

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Output converts upstream Canvas PNG (or away-state black JPEG) and sends it
// through the independent Rev.A serial transport. Pending frames are latest-only.
type Output struct {
	target  func() (string, error)
	logger  *slog.Logger
	open    func(string) (*Conn, error)
	pending chan []byte
	done    chan struct{}

	submitMu sync.Mutex
}

// NewOutput selects the device at send time, so unplug/replug and COM port
// reassignment can be recovered by the next frame.
func NewOutput(target func() (string, error), logger *slog.Logger) *Output {
	if logger == nil {
		logger = slog.Default()
	}
	return &Output{
		target: target, logger: logger, open: Open,
		pending: make(chan []byte, 1), done: make(chan struct{}),
	}
}

// Submit replaces any unsent frame. It copies the caller's buffer so the
// producer may reuse it safely. Concurrent submissions are serialized.
func (o *Output) Submit(frame []byte) {
	copied := bytes.Clone(frame)
	o.submitMu.Lock()
	defer o.submitMu.Unlock()
	select {
	case <-o.pending:
	default:
	}
	o.pending <- copied
}

// Run is owned by one goroutine. It drains the latest queued frame after each
// blocking serial transfer. Errors drop that frame; the next Submit retries
// device discovery and Open. Cancelling always RESETs and closes an open Rev.A
// connection before unblocking Wait.
func (o *Output) Run(ctx context.Context) {
	defer close(o.done)
	var conn *Conn
	var connID string
	var lastFailure string
	fail := func(event string, err error) {
		if err.Error() != lastFailure {
			lastFailure = err.Error()
			o.logger.Warn(event, "cause", err)
		}
	}
	shutdown := func() {
		if conn == nil {
			return
		}
		if err := conn.Shutdown(); err != nil {
			o.logger.Warn("compact_reset_close_failed", "cause", err)
		}
		conn = nil
		connID = ""
	}
	defer shutdown()

	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case data := <-o.pending:
			if ctx.Err() != nil {
				return
			}
			frame, err := frameFromImage(data)
			if err != nil {
				fail("compact_frame_rejected", err)
				continue
			}
			if o.target == nil {
				fail("compact_target_failed", fmt.Errorf("compact output target is nil"))
				continue
			}
			id, err := o.target()
			if err != nil {
				fail("compact_target_failed", err)
				continue
			}
			if conn != nil && connID != id {
				shutdown()
			}
			if id == "" {
				continue
			}
			if conn == nil {
				conn, err = o.open(id)
				if err != nil {
					conn = nil
					fail("compact_open_failed", err)
					continue
				}
				connID = id
			}
			if err := conn.SendFrame(frame); err != nil {
				// SendFrame closes the serial handle on I/O failure. Close is
				// idempotent; never attempt RESET on a broken connection.
				_ = conn.Close()
				conn = nil
				connID = ""
				fail("compact_send_failed", err)
				continue
			}
			lastFailure = ""
		}
	}
}

// Wait returns only after Run has exited and its RESET/Close cleanup completed.
func (o *Output) Wait() { <-o.done }
