package vmserrors

import "errors"

// A command that displays its own messages as it goes -- RENAME shows one
// or more message lines for each file it couldn't rename, and carries on
// with the rest -- still has to fail as a whole, so a one-shot govax
// command exits nonzero. But its messages are already on the console, and
// the console must not show the failure again. VMS has the same need: a
// utility that has signalled its own messages returns its status with
// STS$M_INHIBIT_MSG set, and DCL then displays nothing. InhibitMessage is
// that bit for a Go error.

// inhibitedError is an error whose message has already been displayed.
type inhibitedError struct {
	err error
}

func (e inhibitedError) Error() string { return e.err.Error() }

func (e inhibitedError) Unwrap() error { return e.err }

// InhibitMessage marks err as already displayed (see this file's opening
// comment). A nil err stays nil.
func InhibitMessage(err error) error {
	if err == nil {
		return nil
	}

	return inhibitedError{err: err}
}

// MessageInhibited reports whether err, or an error it wraps, was marked
// by InhibitMessage: whoever would display it should stay quiet.
func MessageInhibited(err error) bool {
	var inhibited inhibitedError

	return errors.As(err, &inhibited)
}
