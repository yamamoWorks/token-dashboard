//go:build windows && !server

// Test driver for the real desktop notification entry point and serial ownership.
package main

import (
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: session-notify PID COM-port lock|unlock|probe")
	}
	pid, err := strconv.ParseUint(os.Args[1], 10, 32)
	if err != nil {
		panic(err)
	}
	if os.Args[3] == "probe" {
		name, err := windows.UTF16PtrFromString(`\\.\` + os.Args[2])
		if err != nil {
			panic(err)
		}
		h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
		switch err {
		case nil:
			windows.CloseHandle(h)
			fmt.Println("free")
		case windows.ERROR_ACCESS_DENIED, windows.ERROR_SHARING_VIOLATION:
			fmt.Println("busy")
		default:
			fmt.Println("unavailable") // Reset may temporarily re-enumerate USB.
		}
		return
	}
	user := windows.NewLazySystemDLL("user32.dll")
	class, _ := windows.UTF16PtrFromString("WailsWebviewWindow")
	title, _ := windows.UTF16PtrFromString("__wails_hidden_mainthread")
	hwnd, _, err := user.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		panic(err)
	}
	var owner uint32
	user.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&owner)))
	if owner != uint32(pid) {
		panic(fmt.Sprintf("unexpected window owner %d", owner))
	}
	var session uint32
	windows.NewLazySystemDLL("kernel32.dll").NewProc("ProcessIdToSessionId").Call(uintptr(pid), uintptr(unsafe.Pointer(&session)))
	msg, event, data := uintptr(0x02B1), uintptr(7), uintptr(session)
	switch os.Args[3] {
	case "lock":
	case "unlock":
		event = 8
	default:
		panic("unknown event")
	}
	var result uintptr
	ok, _, err := user.NewProc("SendMessageTimeoutW").Call(hwnd, msg, event, data, 2, 2000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		panic(err)
	}
}
