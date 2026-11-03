//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package hostlock

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes flock(2)'s exclusive lock on f, without waiting
// (LOCK_NB): EWOULDBLOCK means another open file holds it.
func lockFile(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrLocked
	}

	return err
}
