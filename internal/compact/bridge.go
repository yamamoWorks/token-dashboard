package compact

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"log/slog"
	"strings"
	"sync"
)

// Bridge adapts the unchanged display.Run callbacks to the Rev.A PNG output.
// It has no worker or frame queue: Output remains the sole transport owner.
type Bridge struct {
	mu      sync.Mutex
	ctx     context.Context
	preview func() string
	submit  func([]byte)
	logger  *slog.Logger
}

func NewBridge(ctx context.Context, preview func() string, submit func([]byte), logger *slog.Logger) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &Bridge{ctx: ctx, preview: preview, submit: submit, logger: logger}
}

// OnOutput discards normal JPEGs. Only a fully decoded, entirely black portrait
// JPEG may pass as an away instruction; its pixels never become picture data.
func (b *Bridge) OnOutput(data []byte) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx.Err() != nil || b.submit == nil {
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || config.Width != sourceHeight || config.Height != sourceWidth {
		return
	}
	if _, err := frameFromImage(data); err != nil {
		return
	}
	if b.ctx.Err() == nil {
		b.submit(data)
	}
}

// OnUpdated must run synchronously in the Go display.Updated emit wrapper.
// display.Run stores its PNG in Preview before emitting that event, including
// the first fresh frame after returning from away. No UI event round trip is used.
func (b *Bridge) OnUpdated() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx.Err() != nil || b.preview == nil || b.submit == nil {
		return
	}
	preview := b.preview()
	if preview == "" {
		return
	}
	encoded, ok := strings.CutPrefix(preview, "data:image/png;base64,")
	if !ok {
		b.logger.Warn("compact_preview_rejected", "cause", "expected PNG data URL")
		return
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		b.logger.Warn("compact_preview_rejected", "cause", "invalid base64")
		return
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "png" {
		b.logger.Warn("compact_preview_rejected", "cause", "expected PNG image")
		return
	}
	if _, err := frameFromImage(data); err != nil {
		b.logger.Warn("compact_preview_rejected", "cause", err)
		return
	}
	if b.ctx.Err() == nil {
		b.submit(data)
	}
}
