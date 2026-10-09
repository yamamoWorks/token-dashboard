//go:build windows

package turzx

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRevASendFrameWritesStrips(t *testing.T) {
	var read, write windows.Handle
	if err := windows.CreatePipe(&read, &write, nil, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(read) })
	s := &revASender{handle: write}
	t.Cleanup(func() { s.Close() })
	received := make(chan []byte)
	go func() {
		var all []byte
		buf := make([]byte, 64*1024)
		for {
			var n uint32
			if err := windows.ReadFile(read, buf, &n, nil); err != nil || n == 0 {
				break
			}
			all = append(all, buf[:n]...)
		}
		received <- all
	}()
	sendErr := s.SendFrame(revATestFrame(image.Point{}))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-received
	if sendErr != nil {
		t.Fatal(sendErr)
	}
	var want []byte
	for i := 0; i < 60; i++ {
		x := i * 8
		want = append(want, revABitmapCommand(x, 0, x+7, 319)...)
		want = append(want, revATestStrip(image.Rect(x, 0, x+8, revAHeight))...)
	}
	if len(got) != 60*(6+5120) {
		t.Fatalf("wrote %d bytes, want 307560", len(got))
	}
	if !bytes.Equal(got, want) {
		t.Fatal("written bytes differ from the expected strip sequence")
	}
}

func TestRevASendFrameRejectsWrongSizeWithoutWriting(t *testing.T) {
	s := &revASender{}
	err := s.SendFrame(image.NewRGBA(image.Rect(0, 0, 320, 480)))
	if err == nil || !strings.Contains(err.Error(), "must be 480x320") {
		t.Fatalf("SendFrame error = %v, want size error", err)
	}
}
