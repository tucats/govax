//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package hostlock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestTryLock checks that a second lock on a held file is refused, that
// closing the first lets a new one be taken, and that a missing file is
// an error.
func TestTryLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "volume.dsk")
	if err := os.WriteFile(path, []byte("blocks"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := TryLock(path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	if _, err := TryLock(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock = %v, want ErrLocked", err)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	if err := first.Close(); err != nil {
		t.Errorf("closing twice: %v", err)
	}

	again, err := TryLock(path)
	if err != nil {
		t.Fatalf("lock after the first was closed: %v", err)
	}

	_ = again.Close()

	if _, err := TryLock(filepath.Join(t.TempDir(), "missing.dsk")); err == nil || errors.Is(err, ErrLocked) {
		t.Errorf("lock of a missing file = %v", err)
	}

	var none *Lock
	if err := none.Close(); err != nil {
		t.Errorf("nil Lock's Close = %v", err)
	}
}
