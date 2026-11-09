package lbrrtl

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Holding a library open (subtask 11d, round 3b). A library is read whole
// when LBR$OPEN opens it and written back by LBR$CLOSE, but while it's
// open its file is held as VMS 7.1's LBRSHR held it (testdata/help90's
// LBR90X, cases 12 to 17): opened to update, no one else may read or
// write it; opened to read, others may read it but not write it.
//
//   - An open that conflicts with another process's hold waits until the
//     hold goes (X13 and X14 waited 76 seconds, until X12 closed WX, and
//     then opened it). The manual says LBR$OPEN retries every second for 30
//     seconds and then returns RMS$_FLK; VMS 7.1's in one image gave FLK
//     after 120 seconds, so govax waits up to 120 (openRetry; unconfirmed
//     for another process, whose holder never held it that long).
//   - An open that conflicts with the image's own hold can't succeed, and
//     is RMS$_FLK with the status value SS$_ACCONFLICT, the control index
//     gone (X15, X16, and round 3's case 31). VMS 7.1 took its 120 seconds
//     to say so; govax says so at once.
//   - Two opens to read share the file (X17).
//
// A hold is let go before the routines write the file (LBR$CLOSE, and
// LBR$FLUSH, which takes it again after), and at image rundown.

// openRetry is how long LBR$OPEN waits for another process's hold to go.
const openRetry = 120 * vmsdef.TicksPerSecond

// ssAccConflict is SS$_ACCONFLICT, a locked library's status value.
const ssAccConflict = 0x00000800

// holdLibrary holds the library c opens, at found: 0 and no error when
// it's held (or can't be: a host file), RMS$_FLK when it can't be, and
// corevms.ErrWait while it waits for another process's hold to go, after
// which LBR$OPEN runs again from its start.
func holdLibrary(env *corevms.Environment, c *control, n uint32, found rms.FileLocation) (uint32, error) {
	if found.Host || env.Session == nil {
		c.holdUntil = 0

		return 0, nil
	}

	st := stateOf(env)
	update := c.function == funcUpdate

	f, err := env.Session.HoldFile(found, update)

	switch {
	case err == nil:
		c.held, c.heldName, c.holdUntil = f, found.Name, 0

		return 0, nil
	case !errors.Is(err, rms.ErrACPAccessConflict):
		c.holdUntil = 0

		return 0, nil // not held, as before holds
	}

	if st.holds(found.Name) || (c.holdUntil != 0 && env.Clock() >= c.holdUntil) {
		delete(st.controls, n)
		st.stv = ssAccConflict

		return rmsFLK, nil
	}

	if c.holdUntil == 0 {
		c.holdUntil = env.Clock() + openRetry
	}

	// The wait's test takes the hold and lets it go again: LBR$OPEN, run
	// again, reads the file before it holds it.
	return 0, env.WaitFor(func() bool {
		f, err := env.Session.HoldFile(found, update)
		if err != nil {
			return false
		}

		_ = f.Deaccess()

		return true
	}, c.holdUntil)
}

// holds reports whether one of the image's control indexes holds the file
// name names.
func (st *state) holds(name string) bool {
	for _, o := range st.controls {
		if o.held != nil && strings.EqualFold(o.heldName, name) {
			return true
		}
	}

	return false
}

// release lets c's hold go.
func (c *control) release() {
	if c.held != nil {
		_ = c.held.Deaccess()
		c.held = nil
	}
}

// Rundown lets every hold go at image rundown (corevms.ImageRundowner).
func (st *state) Rundown() {
	for _, c := range st.controls {
		c.release()
	}
}
