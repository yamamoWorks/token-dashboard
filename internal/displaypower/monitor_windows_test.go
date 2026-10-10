//go:build windows && !server

package displaypower

import (
	"io"
	"log/slog"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDisplayState(t *testing.T) {
	tests := []struct {
		name string
		guid windows.GUID
		length uint32
		state uint32
		on bool
		ok bool
	}{
		{"off", sessionDisplayStatus, 4, 0, false, true},
		{"on", sessionDisplayStatus, 4, 1, true, true},
		{"dimmed", sessionDisplayStatus, 4, 2, false, false},
		{"unknown", sessionDisplayStatus, 4, 3, false, false},
		{"wrong length", sessionDisplayStatus, 0, 1, false, false},
		{"unrelated guid", windows.GUID{}, 4, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := powerBroadcastSetting{PowerSetting: tt.guid, DataLength: tt.length}
			*(*uint32)(unsafe.Pointer(&p.Data[0])) = tt.state
			got, ok := displayState(uintptr(unsafe.Pointer(&p)))
			if got != tt.on || ok != tt.ok {
				t.Fatalf("displayState = (%v,%v), want (%v,%v)", got, ok, tt.on, tt.ok)
			}
		})
	}
	if _, ok := displayState(0); ok { t.Fatal("nil power event should be ignored") }
}

func TestMonitorIgnoresEventsWithoutRegistration(t *testing.T) {
	called := 0
	m := New(func(bool) { called++ }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := powerBroadcastSetting{PowerSetting: sessionDisplayStatus, DataLength: 4}
	m.Intercept(1, wmPowerBroadcast, pbtPowerSettingChange, uintptr(unsafe.Pointer(&p)))
	if called != 0 { t.Fatal("unregistered HWND should be ignored") }
}
