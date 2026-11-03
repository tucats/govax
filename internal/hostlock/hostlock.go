// Package hostlock takes an advisory lock on a host file, so that two
// govax sessions don't both write one ODS-2 container (docs/PHASE-87 -
// system volume.md, Decision 7).
//
// A container is a volume's blocks in a host file. A session that mounts
// it keeps its own picture of the volume in memory (the allocation
// bitmap, file headers, directories), and writes it back as it goes. Two
// sessions writing one container would each overwrite what the other
// wrote, and the volume would soon be damaged. So a session that mounts a
// container for writing first takes this lock on it; a second session
// finds it taken, and mounts the container read-only instead.
//
// The lock is "advisory": it stops only programs that ask for it, which
// govax always does, and it doesn't stop a read. It belongs to the open
// file it was taken through, so it goes when Close is called or when the
// process ends, however it ends: a session that crashes leaves no lock
// behind. Two Locks on one file conflict even within one process, so a
// test can play two sessions.
//
// On Unix (macOS, Linux, the BSDs) it is flock(2). On other systems
// TryLock always succeeds and locks nothing (lock_other.go).
package hostlock

import (
	"errors"
	"os"
)

// ErrLocked is TryLock's error when another Lock holds the file.
var ErrLocked = errors.New("in use by another govax session")

// Lock is a lock held on a host file, until Close.
type Lock struct {
	f *os.File
}

// TryLock takes an exclusive lock on the existing file at path, without
// waiting: if another Lock holds it, it returns ErrLocked at once.
func TryLock(path string) (*Lock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	if err := lockFile(f); err != nil {
		_ = f.Close()

		return nil, err
	}

	return &Lock{f: f}, nil
}

// Close lets the lock go. A nil Lock, or one already closed, is
// nothing to close.
func (l *Lock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}

	f := l.f
	l.f = nil

	return f.Close() // closing the file releases its lock
}
