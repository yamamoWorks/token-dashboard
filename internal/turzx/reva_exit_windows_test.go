//go:build windows

package turzx

import (
	"bytes"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRevAExitWritesReset(t *testing.T) {
	var read, write windows.Handle
	if err := windows.CreatePipe(&read, &write, nil, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(read) })
	s := &revASender{handle: write}
	t.Cleanup(func() { s.Close() })
	if err := s.Exit(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	var got [16]byte
	var n uint32
	if err := windows.ReadFile(read, got[:], &n, nil); err != nil {
		t.Fatal(err)
	}
	if want := []byte{0, 0, 0, 0, 0, 101}; !bytes.Equal(got[:n], want) {
		t.Fatalf("Exit wrote % X, want % X", got[:n], want)
	}
}

func TestRevAExitReturnsWriteFailure(t *testing.T) {
	s := &revASender{}
	if err := s.Exit(); err == nil {
		t.Fatal("Exit succeeded with an invalid handle")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after failed Exit: %v", err)
	}
}
