//go:build windows && !server

// Package displaypower listens for Windows session-display power notifications.
package displaypower

import (
	"log/slog"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmCreate             = 0x0001
	wmNCDestroy          = 0x0082
	wmPowerBroadcast     = 0x0218
	pbtAPMSuspend        = 0x0004
	pbtPowerSettingChange = 0x8013
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	// Copying from the WM_POWERBROADCAST lParam via a Win32 call avoids
	// converting an untracked uintptr into a Go unsafe.Pointer.
	moveMemory = windows.NewLazySystemDLL("kernel32.dll").NewProc("RtlMoveMemory")
	registerPowerSettingNotification = user32.NewProc("RegisterPowerSettingNotification")
	unregisterPowerSettingNotification = user32.NewProc("UnregisterPowerSettingNotification")
	sessionDisplayStatus = windows.GUID{
		Data1: 0x2B84C20E, Data2: 0xAD23, Data3: 0x4DDF,
		Data4: [8]byte{0x93, 0xDB, 0x05, 0xFF, 0xBD, 0x7E, 0xFC, 0xA5},
	}
)

type powerBroadcastSetting struct {
	PowerSetting windows.GUID
	DataLength   uint32
	Data         [4]byte
}

// Monitor intercepts the Wails window procedure. It uses the first created
// application HWND for registration; Wails keeps its main-thread HWND alive
// even when the visible settings window is hidden in the tray.
type Monitor struct {
	mu     sync.Mutex
	hwnd   uintptr
	handle uintptr
	logger *slog.Logger
	onDisplay func(bool)
}

func New(onDisplay func(bool), logger *slog.Logger) *Monitor {
	return &Monitor{onDisplay: onDisplay, logger: logger}
}

// Intercept observes messages but lets Wails handle every message normally.
func (m *Monitor) Intercept(hwnd uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
	switch msg {
	case wmCreate:
		m.mu.Lock()
		if m.handle == 0 && hwnd != 0 {
			handle, _, err := registerPowerSettingNotification.Call(hwnd, uintptr(unsafe.Pointer(&sessionDisplayStatus)), 0)
			if handle == 0 {
				m.logger.Warn("display_power_registration_failed", "cause", err)
			} else {
				m.hwnd, m.handle = hwnd, handle
			}
		}
		m.mu.Unlock()
	case wmNCDestroy:
		m.mu.Lock()
		if hwnd == m.hwnd {
			m.unregister()
		}
		m.mu.Unlock()
	case wmPowerBroadcast:
		// Handle only messages delivered to the registered HWND.
		m.mu.Lock()
		registered := hwnd == m.hwnd && m.handle != 0
		m.mu.Unlock()
		if !registered {
			break
		}
		switch wParam {
		case pbtAPMSuspend:
			// Best effort; a machine may suspend before the reset completes.
			m.onDisplay(false)
		case pbtPowerSettingChange:
			if on, ok := displayState(lParam); ok {
				m.onDisplay(on)
			}
		}
	}
	return 0, false
}

// displayState recognizes OFF and ON; DIMMED(2) and unrelated GUIDs are ignored.
func displayState(lParam uintptr) (on bool, ok bool) {
	if lParam == 0 {
		return false, false
	}
	// The fixed POWERBROADCAST_SETTING prefix is 20 bytes; validate its
	// GUID and length before reading the following 4-byte display state.
	var header struct {
		PowerSetting windows.GUID
		DataLength uint32
	}
	moveMemory.Call(uintptr(unsafe.Pointer(&header)), lParam, unsafe.Sizeof(header))
	if header.PowerSetting != sessionDisplayStatus || header.DataLength != 4 {
		return false, false
	}
	var state uint32
	moveMemory.Call(uintptr(unsafe.Pointer(&state)), lParam+unsafe.Sizeof(header), 4)
	switch state {
	case 0:
		return false, true
	case 1:
		return true, true
	default:
		return false, false
	}
}

// unregister must be called with m.mu held.
func (m *Monitor) unregister() {
	if m.handle != 0 {
		if r, _, err := unregisterPowerSettingNotification.Call(m.handle); r == 0 {
			m.logger.Warn("display_power_unregistration_failed", "cause", err)
		}
	}
	m.handle, m.hwnd = 0, 0
}

func (m *Monitor) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unregister()
}
