//go:build windows

package compact

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"token-monitor-turzx/internal/turzx"
)

const (
	regRevAParameters = `SYSTEM\CurrentControlSet\Enum\USB\VID_1A86&PID_5722\USB35INCHIPSV2\Device Parameters`
	baud115200        = 115200
	fBinary           = 1 << 0
	fOutxCTSFlow      = 1 << 2
	rtsHandshake      = 2 << 12
	crSuccess         = 0
	crNoSuchDevnode   = 0x0D
)

var procLocateRevA = syscall.NewLazyDLL("cfgmgr32.dll").NewProc("CM_Locate_DevNodeW")

func devicePresent() (bool, error) {
	name, err := syscall.UTF16PtrFromString(RevAID)
	if err != nil {
		return false, err
	}
	var node uint32
	r, _, _ := procLocateRevA.Call(uintptr(unsafe.Pointer(&node)), uintptr(unsafe.Pointer(name)), 0)
	switch r {
	case crSuccess:
		return true, nil
	case crNoSuchDevnode:
		return false, nil
	default:
		return false, fmt.Errorf("locate TURZX Rev.A USB device: CONFIGRET=0x%X", r)
	}
}

func validCOMPort(port string) bool {
	if len(port) < 4 || !strings.EqualFold(port[:3], "COM") {
		return false
	}
	number, err := strconv.Atoi(port[3:])
	return err == nil && number > 0 && strconv.Itoa(number) == port[3:]
}

// portName reads the current COM assignment on each call (no stale port cache).
func portName() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regRevAParameters, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("locate TURZX Rev.A COM parameters: %w", err)
	}
	defer key.Close()
	port, _, err := key.GetStringValue("PortName")
	if err != nil {
		return "", fmt.Errorf("read TURZX Rev.A COM port: %w", err)
	}
	if !validCOMPort(port) {
		return "", fmt.Errorf("invalid TURZX Rev.A COM port %q", port)
	}
	return strings.ToUpper(port), nil
}

// ListDevices enumerates only the connected 3.5-inch Rev.A instance.
func ListDevices() ([]turzx.Device, error) {
	present, err := devicePresent()
	if err != nil {
		return nil, err
	}
	if !present {
		return []turzx.Device{}, nil
	}
	port, err := portName()
	if err != nil {
		return nil, err
	}
	return []turzx.Device{{ID: RevAID, Name: "TURZX 3.5 Inch Rev.A (" + port + ")"}}, nil
}

// Open rediscovers the live COM assignment, opens an exclusive serial handle,
// configures 115200 8N1 with RTS/CTS, and initializes landscape orientation.
func Open(id string) (*Conn, error) {
	if !strings.EqualFold(id, RevAID) {
		return nil, fmt.Errorf("unsupported TURZX Rev.A device ID %q", id)
	}
	present, err := devicePresent()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("TURZX Rev.A %s is disconnected", id)
	}
	port, err := portName()
	if err != nil {
		return nil, err
	}
	name, err := syscall.UTF16PtrFromString(`\\.\` + port)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("open TURZX Rev.A serial port %s: %w", port, err)
	}
	serial := &serialPort{handle: h}
	if err := configureSerial(h); err != nil {
		return nil, errors.Join(fmt.Errorf("configure TURZX Rev.A serial port %s: %w", port, err), serial.Close())
	}
	return newConn(serial) // newConn closes serial on orientation failure.
}

func configureSerial(h windows.Handle) error {
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(h, &dcb); err != nil {
		return fmt.Errorf("read serial settings: %w", err)
	}
	dcb.BaudRate = baud115200
	dcb.ByteSize, dcb.Parity, dcb.StopBits = 8, 0, 0
	dcb.Flags = fBinary | fOutxCTSFlow | rtsHandshake
	if err := windows.SetCommState(h, &dcb); err != nil {
		return fmt.Errorf("set 115200 8N1 RTS/CTS: %w", err)
	}
	if err := windows.SetCommTimeouts(h, &windows.CommTimeouts{WriteTotalTimeoutConstant: 3000}); err != nil {
		return fmt.Errorf("set serial write timeout: %w", err)
	}
	return nil
}

type serialPort struct {
	handle windows.Handle
	closed bool
}

func (s *serialPort) Write(p []byte) (int, error) {
	if s.closed {
		return 0, fmt.Errorf("TURZX Rev.A serial port is closed")
	}
	if len(p) == 0 {
		return 0, nil
	}
	var n uint32
	if err := windows.WriteFile(s.handle, p, &n, nil); err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (s *serialPort) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return windows.CloseHandle(s.handle)
}
