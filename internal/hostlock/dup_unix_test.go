//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package hostlock

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestCloseWithCopy: a lock let go by Close is free at once even while
// another descriptor for the same open file exists, as a child process holds
// one between its fork and its exec. A duplicated descriptor shares the open
// file just as that copy does.
func TestCloseWithCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "volume.dsk")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	lock, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}

	copied, err := syscall.Dup(int(lock.f.Fd()))
	if err != nil {
		t.Fatal(err)
	}

	defer syscall.Close(copied)

	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock after Close, with a copy of the descriptor open: %v", err)
	}

	_ = again.Close()
}
