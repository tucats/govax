//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package hostlock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

// TestWait checks that Wait makes a missing file, that a second Wait
// waits until the first lock goes, and that TryLock is refused meanwhile.
func TestWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sysuaf.json.lock")

	first, err := Wait(path)
	if err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Wait made no file: %v", err)
	}

	if _, err := TryLock(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("TryLock while held = %v, want ErrLocked", err)
	}

	got := make(chan error, 1)

	go func() {
		second, err := Wait(path)
		if err == nil {
			err = second.Close()
		}

		got <- err
	}()

	select {
	case err := <-got:
		t.Fatalf("second Wait returned (%v) while the first held the lock", err)
	case <-time.After(50 * time.Millisecond):
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("second Wait: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Wait never got the lock")
	}
}
