//go:build windows

package turzx

import (
	"fmt"
	"image"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	baud115200          = 115200
	fBinary             = 1 << 0
	fOutxCTSFlow        = 1 << 2
	fRtsControlMask     = 3 << 12
	rtsControlHandshake = 2 << 12
)

type revASender struct{ handle windows.Handle }

func openRevASender(id string) (*revASender, error) {
	if !IsRevA(id) {
		return nil, fmt.Errorf("unsupported TURZX device ID %q", id)
	}
	port, err := revAPortName()
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
	if err := configureRevASerial(h); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	s := &revASender{handle: h}
	if err := s.write(revAOrientation()); err != nil {
		s.Close()
		return nil, fmt.Errorf("set TURZX Rev.A landscape orientation: %w", err)
	}
	return s, nil
}

func openFrameSender(id string) (FrameSender, error) {
	if IsRevA(id) {
		return openRevASender(id)
	}
	c, err := Open(id)
	if err != nil {
		return nil, err
	}
	return &jpegFrameSender{conn: c}, nil
}

func revAPortName() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\USB\VID_1A86&PID_5722\USB35INCHIPSV2\Device Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("find TURZX Rev.A PNP instance: %w", err)
	}
	defer key.Close()
	port, _, err := key.GetStringValue("PortName")
	if err != nil || !strings.HasPrefix(strings.ToUpper(port), "COM") {
		return "", fmt.Errorf("TURZX Rev.A COM port is unavailable")
	}
	return port, nil
}

func configureRevASerial(h windows.Handle) error {
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(h, &dcb); err != nil {
		return fmt.Errorf("read TURZX Rev.A serial settings: %w", err)
	}
	dcb.BaudRate = baud115200
	dcb.ByteSize, dcb.Parity, dcb.StopBits = 8, 0, 0
	dcb.Flags = fBinary | fOutxCTSFlow | rtsControlHandshake
	if err := windows.SetCommState(h, &dcb); err != nil {
		return fmt.Errorf("configure TURZX Rev.A serial settings: %w", err)
	}
	timeouts := windows.CommTimeouts{WriteTotalTimeoutConstant: 3000}
	if err := windows.SetCommTimeouts(h, &timeouts); err != nil {
		return fmt.Errorf("set TURZX Rev.A serial timeout: %w", err)
	}
	return nil
}

func (s *revASender) write(data []byte) error {
	for len(data) > 0 {
		var written uint32
		if err := windows.WriteFile(s.handle, data, &written, nil); err != nil {
			return err
		}
		if written == 0 {
			return fmt.Errorf("TURZX Rev.A serial write made no progress")
		}
		data = data[written:]
	}
	return nil
}

func (s *revASender) SendFrame(img *image.RGBA) error {
	pixels, err := revARGB565LE(img)
	if err != nil {
		return err
	}
	if err := s.write(revABitmapCommand(revAWidth, revAHeight)); err != nil {
		return fmt.Errorf("send TURZX Rev.A bitmap command: %w", err)
	}
	for _, chunk := range revAChunks(pixels) {
		if err := s.write(chunk); err != nil {
			return fmt.Errorf("send TURZX Rev.A image data: %w", err)
		}
	}
	return nil
}

func (s *revASender) Exit() error { return s.write(revAResetCommand()) }
func (s *revASender) Close() error {
	if s.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(s.handle)
	s.handle = 0
	return err
}
