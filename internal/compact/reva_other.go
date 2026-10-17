//go:build !windows

package compact

import (
	"fmt"

	"token-monitor-turzx/internal/turzx"
)

// ListDevices finds no Rev.A serial hardware outside Windows.
func ListDevices() ([]turzx.Device, error) { return []turzx.Device{}, nil }

// Open is available only on Windows, where the Rev.A device is a COM port.
func Open(id string) (*Conn, error) {
	return nil, fmt.Errorf("TURZX Rev.A serial port is supported only on Windows (id %q)", id)
}
