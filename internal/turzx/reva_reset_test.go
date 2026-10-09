package turzx

import (
	"bytes"
	"testing"
)

func TestRevAResetCommand(t *testing.T) {
	want := []byte{0, 0, 0, 0, 0, 101}
	if got := revAResetCommand(); !bytes.Equal(got, want) {
		t.Fatalf("reset command = %v, want %v", got, want)
	}
}
