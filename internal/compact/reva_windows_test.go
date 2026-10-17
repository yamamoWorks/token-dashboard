//go:build windows

package compact

import (
	"image"
	"image/color"
	"os"
	"testing"
)

func TestCOMPortValidation(t *testing.T) {
	for _, port := range []string{"COM1", "com9", "COM12", "COM255"} {
		if !validCOMPort(port) {
			t.Errorf("rejected %q", port)
		}
	}
	for _, port := range []string{"", "COM", "COM0", "COM-1", "COM1x", "COM01", "LPT1", "COM1\\bad"} {
		if validCOMPort(port) {
			t.Errorf("accepted %q", port)
		}
	}
}

// Run with TURZX_REVA_DEVICE_TEST=1 on a Windows PC with Rev.A connected.
// This sends a solid-red test frame and then RESETs the controller.
func TestRevADeviceIntegration(t *testing.T) {
	if os.Getenv("TURZX_REVA_DEVICE_TEST") != "1" {
		t.Skip("requires a connected Rev.A and TURZX_REVA_DEVICE_TEST=1")
	}
	devices, err := ListDevices()
	if err != nil || len(devices) != 1 || devices[0].ID != RevAID {
		t.Fatalf("ListDevices = %v, %v", devices, err)
	}
	c, err := Open(devices[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	frame := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			frame.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	if err := c.SendFrame(frame); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
}
