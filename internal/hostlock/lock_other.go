//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package hostlock

import "os"

// lockFile locks nothing on a system without flock(2): every TryLock
// succeeds, so two sessions there can still both write one container.
// (Windows's LockFileEx is follow-up work: docs/planning/PHASE-87 - system
// volume.md.)
func lockFile(*os.File) error {
	return nil
}
