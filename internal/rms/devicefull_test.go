package rms

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeviceFullFromODS2: a host file too big for the 400-block test volume,
// copied from the host and from the volume, fails with a *DeviceFullError
// naming the output and the version it was made with. The volume's own
// words are ods2's (ods2NoSpace), which DeviceFull looks for, so this test
// fails if ods2 words them otherwise.
func TestDeviceFullFromODS2(t *testing.T) {
	s, _ := newCopyTestSession(t)

	hostPath := filepath.Join(t.TempDir(), "big.txt")
	line := strings.Repeat("X", 79) + "\n"

	if err := os.WriteFile(hostPath, []byte(strings.Repeat(line, 4000)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := s.Copy(hostPath, true, "BIG.TXT", false, CopyOptions{})

	var full *DeviceFullError
	if !errors.As(err, &full) || !DeviceFull(err) {
		t.Fatalf("Copy = %v, want a *DeviceFullError", err)
	}

	if full.Spec != "DUA0:[000000]BIG.TXT;1" {
		t.Errorf("Spec = %q, want DUA0:[000000]BIG.TXT;1", full.Spec)
	}

	if !strings.Contains(full.Err.Error(), ods2NoSpace) {
		t.Errorf("the volume's words %q no longer say %q", full.Err, ods2NoSpace)
	}

	// The partial file stays (govax's choice, unconfirmed), so the volume
	// is now full: a copy of a volume file to it fails the same way.
	_, err = s.Copy("FOO.TXT", false, "MORE.TXT", false, CopyOptions{})
	if !DeviceFull(err) {
		t.Errorf("Copy(FOO.TXT) = %v, want a full device", err)
	}

	if DeviceFull(nil) || DeviceFull(errors.New("something else")) {
		t.Error("DeviceFull of nil or another error is true")
	}
}
