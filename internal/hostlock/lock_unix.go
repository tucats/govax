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

// waitFile takes flock(2)'s exclusive lock on f, waiting while another
// open file holds it.
func waitFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// unlockFile lets flock(2)'s lock on f go (LOCK_UN), for the open file
// itself, so that no copy of its descriptor keeps it held.
func unlockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
