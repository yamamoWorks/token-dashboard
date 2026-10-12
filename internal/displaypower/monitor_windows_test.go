//go:build windows && !server

package displaypower

import (
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDisplayState(t *testing.T) {
	tests := []struct {
		name   string
		guid   windows.GUID
		length uint32
		state  uint32
		on     bool
		ok     bool
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
	if _, ok := displayState(0); ok {
		t.Fatal("nil power event should be ignored")
	}
}

func TestMonitorIgnoresEventsWithoutRegistration(t *testing.T) {
	called := 0
	m := New(func(bool) { called++ }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := powerBroadcastSetting{PowerSetting: sessionDisplayStatus, DataLength: 4}
	m.Intercept(1, wmPowerBroadcast, pbtPowerSettingChange, uintptr(unsafe.Pointer(&p)))
	m.Intercept(1, wmWTSSessionChange, wtsSessionLock, 0)
	if called != 0 {
		t.Fatal("unregistered HWND should be ignored")
	}
}

func TestMonitorCombinesLockAndDisplayState(t *testing.T) {
	var got []bool
	m := New(func(on bool) { got = append(got, on) }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Registration is covered by the real desktop E2E. Isolate ordering here.
	m.hwnd, m.handle, m.sessionRegistered = 1, 1, true
	p := powerBroadcastSetting{PowerSetting: sessionDisplayStatus, DataLength: 4}
	power := func(state uint32) {
		*(*uint32)(unsafe.Pointer(&p.Data[0])) = state
		m.Intercept(1, wmPowerBroadcast, pbtPowerSettingChange, uintptr(unsafe.Pointer(&p)))
		runtime.KeepAlive(&p)
	}
	m.Intercept(2, wmWTSSessionChange, wtsSessionLock, 0)
	m.Intercept(1, wmWTSSessionChange, 6, 0) // Logoff is not lock.
	if len(got) != 0 {
		t.Fatalf("unrelated events changed transfer: %v", got)
	}
	m.Intercept(1, wmWTSSessionChange, wtsSessionLock, 0)
	m.Intercept(1, wmWTSSessionChange, wtsSessionLock, 0)
	power(1)
	power(0)
	m.Intercept(1, wmWTSSessionChange, wtsSessionUnlock, 0)
	m.Intercept(1, wmWTSSessionChange, wtsSessionUnlock, 0)
	if !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("resumed before ON: %v", got)
	}
	power(1)
	power(1)
	// OFF followed by lock must not trigger another reset; ON while locked waits.
	power(0)
	m.Intercept(1, wmWTSSessionChange, wtsSessionLock, 0)
	power(1)
	m.Intercept(1, wmWTSSessionChange, wtsSessionUnlock, 0)
	if !reflect.DeepEqual(got, []bool{false, true, false, true}) {
		t.Fatalf("transfer transitions = %v", got)
	}
}
